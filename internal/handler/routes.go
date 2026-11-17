package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pr5_minenko/internal/config"
)

func NewRouter(db *gorm.DB, cfg config.Config) *gin.Engine {
	r := gin.Default()

	incidents := NewIncidentHandler(db)
	authH := NewAuthHandler(db, cfg.JWTSecret, cfg.JWTTTL)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		authGroup := v1.Group("/auth")
		authGroup.POST("/register", authH.RegisterHandler)
		authGroup.POST("/login", authH.LoginHandler)

		inc := v1.Group("/incidents")
		inc.GET("", incidents.ListIncidentsHandler)
		inc.POST("", incidents.CreateIncidentHandler)
		inc.GET("/:id", incidents.GetIncidentHandler)
		inc.PATCH("/:id/status", incidents.UpdateStatusHandler)
		inc.DELETE("/:id", incidents.DeleteIncidentHandler)
	}
	return r
}
