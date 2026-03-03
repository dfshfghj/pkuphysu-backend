package utils

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func RespondError(c *gin.Context, code int, errid string, err error) {
	c.JSON(code, gin.H{
		"status":  code,
		"errid":   errid,
		"message": err.Error(),
	})
}

func RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"status": 200,
		"data":   data,
	})
}
