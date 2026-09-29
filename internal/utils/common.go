package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	message := ""
	if err != nil {
		message = err.Error()
	}
	c.JSON(code, gin.H{
		"status":  code,
		"errid":   errid,
		"message": message,
	})
}

func RespondSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"status": 200,
		"data":   data,
	})
}

func EnsureDirExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func SaveFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := EnsureDirExists(dir); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

func IsValidImageType(filename string) bool {
	validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, validExt := range validExtensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

// TruncateString 截取字符串到指定长度，如果超出则添加省略号
func TruncateString(str string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	// 如果字符串长度小于等于最大长度，直接返回
	if len(str) <= maxLen {
		return str
	}

	// 如果字符串长度小于最大长度，也包括省略号的空间
	if maxLen <= 3 {
		return str[:maxLen]
	}

	return str[:maxLen-3] + "..."
}
