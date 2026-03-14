package handles

import (
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

func Markdown(c *gin.Context) {
	var req struct {
		MarkdownText string `json:"markdownText"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	html := utils.MarkdownToHtml(req.MarkdownText)
	c.JSON(200, gin.H{"code": 0,
		"msg":  "",
		"data": html})
}
