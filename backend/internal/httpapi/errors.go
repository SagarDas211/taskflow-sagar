package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func respondValidationError(c *gin.Context, fields map[string]string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":  "validation failed",
		"fields": fields,
	})
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
