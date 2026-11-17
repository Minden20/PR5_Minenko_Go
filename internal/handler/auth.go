package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"pr5_minenko/internal/auth"
	"pr5_minenko/internal/dto"
	"pr5_minenko/internal/model"
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type AuthHandler struct {
	db        *gorm.DB
	jwtSecret string
	jwtTTL    time.Duration
}

func NewAuthHandler(db *gorm.DB, jwtSecret string, jwtTTL time.Duration) *AuthHandler {
	setupValidator()
	return &AuthHandler{db: db, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

func (h *AuthHandler) RegisterHandler(c *gin.Context) {
	var in dto.RegisterDTO
	if err := c.ShouldBindJSON(&in); err != nil {
		respondBindError(c, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[AUTH] bcrypt: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	user := model.User{
		Email:        normalizeEmail(in.Email),
		PasswordHash: string(hash),
	}
	err = h.db.WithContext(c.Request.Context()).Create(&user).Error
	switch {
	case errors.Is(err, gorm.ErrDuplicatedKey):
		respondError(c, http.StatusConflict, "user with this email already exists")
		return
	case err != nil:
		log.Printf("[DB] register: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) LoginHandler(c *gin.Context) {
	var in dto.LoginDTO
	if err := c.ShouldBindJSON(&in); err != nil {
		respondBindError(c, err)
		return
	}

	var user model.User
	err := h.db.WithContext(c.Request.Context()).Where("email = ?", normalizeEmail(in.Email)).First(&user).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("[DB] login: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	hash := []byte(user.PasswordHash)
	if err != nil {
		hash = dummyHash
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(in.Password)) != nil || user.ID == 0 {
		respondError(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := auth.GenerateToken(user.ID, h.jwtSecret, h.jwtTTL)
	if err != nil {
		log.Printf("[AUTH] sign token: %v", err)
		respondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
