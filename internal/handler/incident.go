package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pr5_minenko/internal/dto"
	"pr5_minenko/internal/model"
)

type IncidentHandler struct {
	db *gorm.DB
}

func NewIncidentHandler(db *gorm.DB) *IncidentHandler {
	setupValidator()
	return &IncidentHandler{db: db}
}

func (h *IncidentHandler) CreateIncidentHandler(c *gin.Context) {
	var in dto.CreateIncidentDTO
	if err := c.ShouldBindJSON(&in); err != nil {
		respondBindError(c, err)
		return
	}

	incident := model.Incident{
		Title:       in.Title,
		Description: in.Description,
		Priority:    in.Priority,
		Status:      "open",
	}
	if err := h.db.WithContext(c.Request.Context()).Create(&incident).Error; err != nil {
		h.internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, incident)
}

func (h *IncidentHandler) ListIncidentsHandler(c *gin.Context) {
	incidents := make([]model.Incident, 0)
	if err := h.db.WithContext(c.Request.Context()).Order("id").Find(&incidents).Error; err != nil {
		h.internal(c, err)
		return
	}
	c.JSON(http.StatusOK, incidents)
}

func (h *IncidentHandler) GetIncidentHandler(c *gin.Context) {
	incident, ok := h.find(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, incident)
}

func (h *IncidentHandler) UpdateStatusHandler(c *gin.Context) {
	incident, ok := h.find(c)
	if !ok {
		return
	}

	var in dto.UpdateStatusDTO
	if err := c.ShouldBindJSON(&in); err != nil {
		respondBindError(c, err)
		return
	}

	if err := h.db.WithContext(c.Request.Context()).Model(&incident).Update("status", in.Status).Error; err != nil {
		h.internal(c, err)
		return
	}
	c.JSON(http.StatusOK, incident)
}

func (h *IncidentHandler) DeleteIncidentHandler(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res := h.db.WithContext(c.Request.Context()).Delete(&model.Incident{}, id)
	if res.Error != nil {
		h.internal(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "incident not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *IncidentHandler) find(c *gin.Context) (model.Incident, bool) {
	var incident model.Incident
	id, ok := parseID(c)
	if !ok {
		return incident, false
	}
	err := h.db.WithContext(c.Request.Context()).First(&incident, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		respondError(c, http.StatusNotFound, "incident not found")
		return incident, false
	case err != nil:
		h.internal(c, err)
		return incident, false
	}
	return incident, true
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, "id must be a positive integer")
		return 0, false
	}
	return uint(id), true
}

func (h *IncidentHandler) internal(c *gin.Context, err error) {
	log.Printf("[DB] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	respondError(c, http.StatusInternalServerError, "internal server error")
}
