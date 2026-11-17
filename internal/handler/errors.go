package handler

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var setupOnce sync.Once

func setupValidator() {
	setupOnce.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			v.RegisterTagNameFunc(func(f reflect.StructField) string {
				name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
				if name == "-" || name == "" {
					return f.Name
				}
				return name
			})
		}
	})
}

func respondBindError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string]string, len(ve))
		for _, fe := range ve {
			fields[fe.Field()] = describe(fe)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "fields": fields})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body: " + err.Error()})
}

func describe(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "field is required"
	case "min":
		return fmt.Sprintf("must be at least %s characters long", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters long", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s (got '%v')", fe.Param(), fe.Value())
	case "email":
		return "must be a valid email address"
	default:
		return fmt.Sprintf("failed on '%s' rule", fe.Tag())
	}
}

func respondError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}
