package handles

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func StaticFile(c *gin.Context) {
	filename := c.Param("file")

	filename = strings.TrimPrefix(filename, "/static/")

	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	// 检查路径遍历攻击
	if strings.Contains(filename, "..") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid file path"})
		return
	}

	if strings.Contains(filename, "\\") {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid file path"})
		return
	}

	filePath := filepath.Join("data", "static", filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filePath)
}

func UploadFile(c *gin.Context) {
	const maxFileSize = 5 * 1024 * 1024 // 5MB

	var files []*multipart.FileHeader

	err := c.Request.ParseMultipartForm(maxFileSize)
	if err != nil {
		file, err := c.FormFile("file")
		if err != nil {
			log.Warnf("File upload failed: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "No file uploaded",
			})
			return
		}
		files = []*multipart.FileHeader{file}
	} else {
		// 解析成功，检查 MultipartForm 是否存在
		if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
			// 尝试获取 file[] 数组
			if fileHeaders, ok := c.Request.MultipartForm.File["file"]; ok && len(fileHeaders) > 0 {
				files = fileHeaders
			} else if fileHeaders, ok := c.Request.MultipartForm.File["file[]"]; ok && len(fileHeaders) > 0 {
				// 有些客户端可能发送 file[] 作为字段名
				files = fileHeaders
			} else {
				// 没有多文件，尝试获取单个文件
				file, err := c.FormFile("file")
				if err != nil {
					log.Warnf("File upload failed: %v", err)
					c.JSON(http.StatusBadRequest, gin.H{
						"status": "error",
						"error":  "No file uploaded",
					})
					return
				}
				files = []*multipart.FileHeader{file}
			}
		} else {
			// MultipartForm 解析后仍然有问题，回退到单文件
			file, err := c.FormFile("file")
			if err != nil {
				log.Warnf("File upload failed: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error",
					"error":  "No file uploaded",
				})
				return
			}
			files = []*multipart.FileHeader{file}
		}
	}

	var results []gin.H

	for _, file := range files {
		if file.Size > maxFileSize {
			log.Warnf("File too large: %d bytes, max allowed: %d bytes", file.Size, maxFileSize)
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  "File size must be less than 5MB",
			})
			return
		}

		fileExtension := ""
		if strings.Contains(file.Filename, ".") {
			split := strings.Split(file.Filename, ".")
			fileExtension = split[len(split)-1]
		}

		src, err := file.Open()
		if err != nil {
			log.Errorf("Failed to open uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to open file",
			})
			return
		}
		defer src.Close()

		hasher := md5.New()
		if _, err := io.Copy(hasher, src); err != nil {
			log.Errorf("Failed to calculate MD5 hash: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to calculate file hash",
			})
			return
		}
		md5Hash := hex.EncodeToString(hasher.Sum(nil))

		src2, err := file.Open()
		if err != nil {
			log.Errorf("Failed to reopen uploaded file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to reopen file",
			})
			return
		}
		defer src2.Close()

		uniqueFilename := md5Hash
		if fileExtension != "" {
			uniqueFilename = uniqueFilename + "." + fileExtension
		}

		baseDir := "./data/static/files"
		err = os.MkdirAll(baseDir, 0755)
		if err != nil {
			log.Errorf("Failed to create directory %s: %v", baseDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to create directory",
			})
			return
		}

		filePath := filepath.Join(baseDir, uniqueFilename)
		dst, err := os.Create(filePath)
		if err != nil {
			log.Errorf("Failed to create file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to create file",
			})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src2); err != nil {
			log.Errorf("Failed to save file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to save file",
			})
			return
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Errorf("Failed to get file info for %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to get file info",
			})
			return
		}
		fileSize := fileInfo.Size()

		fileURL := fmt.Sprintf("/files/%s", uniqueFilename)

		log.Infof("File uploaded: original=%s, saved=%s, size=%d", file.Filename, filePath, fileSize)

		results = append(results, gin.H{
			"url":          fileURL,
			"ext":          fileExtension,
			"originalName": file.Filename,
			"status":       "success",
			"size":         fileSize,
			"id":           uniqueFilename,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"files": results,
		"count": len(results),
	})
}
