package utils

import (
	"github.com/gin-gonic/gin"
)

func RespondError(c *gin.Context, code int, errid string, err error) {
    c.JSON(code, gin.H{
		"status": code,
        "errid": errid,
		"message": err.Error(),
    })
}

func RespondSuccess(c *gin.Context, data interface{}) {
    c.JSON(200, gin.H{
		"status": 200,
		"data": data,
    })
}