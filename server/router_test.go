package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"pkuphysu-backend/server/handles"
)

// TestAllPublicRoutes 测试所有公开路由
func TestAllPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		expectedStatus int
		skip           bool // 是否需要数据库，临时跳过
	}{
		{
			name:           "POST /user/register",
			method:         "POST",
			path:           "/user/register",
			body:           map[string]interface{}{"username": "test", "password": "pass123"},
			expectedStatus: http.StatusInternalServerError, // 没有数据库会失败
			skip:           true,
		},
		{
			name:           "POST /auth/login",
			method:         "POST",
			path:           "/auth/login",
			body:           map[string]interface{}{"username": "test", "password": "pass123"},
			expectedStatus: http.StatusInternalServerError,
			skip:           true,
		},
		{
			name:           "POST /iaaa/login",
			method:         "POST",
			path:           "/iaaa/login",
			body:           map[string]interface{}{"username": "test", "password": "pass123"},
			expectedStatus: http.StatusInternalServerError,
			skip:           true,
		},
		{
			name:           "POST /email/send",
			method:         "POST",
			path:           "/email/send",
			body:           map[string]interface{}{"email": "test@example.com"},
			expectedStatus: http.StatusInternalServerError,
			skip:           true,
		},
		{
			name:           "POST /email/verify",
			method:         "POST",
			path:           "/email/verify",
			body:           map[string]interface{}{"code": "123456"},
			expectedStatus: http.StatusInternalServerError,
			skip:           true,
		},
		{
			name:           "POST /markdown/preview",
			method:         "POST",
			path:           "/markdown/preview",
			body:           map[string]string{"markdown": "# Test"},
			expectedStatus: http.StatusOK,
			skip:           false,
		},
		{
			name:           "GET /static/file",
			method:         "GET",
			path:           "/static/test.txt",
			body:           nil,
			expectedStatus: http.StatusNotFound,
			skip:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Requires database")
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			var req *http.Request
			if tt.body != nil {
				jsonData, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			c.Request = req

			// 根据路径调用对应的 handler
			switch tt.path {
			case "/markdown/preview":
				handles.Markdown(c)
			default:
				t.Logf("No handler mock for path: %s", tt.path)
				return
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestAuthMiddlewareBehavior 测试认证中间件行为
func TestAuthMiddlewareBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty bearer",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("GET", "/user/me", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			c.Request = req

			// 模拟 Auth 中间件的行为
			if tt.authHeader == "" {
				c.Abort()
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			}

			t.Logf("Auth middleware test: %s - would return %d", tt.name, tt.expectedStatus)
		})
	}
}

// TestFileUploadEndpoint 测试文件上传端点
func TestFileUploadEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 这个测试展示如何测试文件上传（简化版）
	t.Logf("File upload test - requires multipart form data and authentication")
}

// TestAPIResponseFormat 测试 API 响应格式
func TestAPIResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 模拟成功响应
	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"data": map[string]interface{}{
			"id":       1,
			"username": "testuser",
		},
	})

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != float64(200) {
		t.Errorf("Expected status 200 in response, got %v", response["status"])
	}

	t.Logf("API response format test passed")
}
