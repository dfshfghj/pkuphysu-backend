package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pkuphysu-backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestGetClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		headers      map[string]string
		expectedIP   string
		trustedProxy bool
		description  string
	}{
		{
			name:         "直接访问无代理",
			headers:      map[string]string{},
			expectedIP:   "192.0.2.1", // httptest 默认的 RemoteAddr
			trustedProxy: true,
			description:  "没有代理 header 时应该使用 ClientIP",
		},
		{
			name: "有 X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178",
			},
			expectedIP:   "203.0.113.195",
			trustedProxy: true,
			description:  "应该提取 X-Forwarded-For 中的第一个 IP",
		},
		{
			name: "有 X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.195",
			},
			expectedIP:   "203.0.113.195",
			trustedProxy: true,
			description:  "应该使用 X-Real-IP",
		},
		{
			name: "同时有 XFF 和 XRI",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195, 70.41.3.18",
				"X-Real-IP":       "198.51.100.178",
			},
			expectedIP:   "203.0.113.195",
			trustedProxy: true,
			description:  "X-Forwarded-For 优先级高于 X-Real-IP",
		},
		{
			name: "伪造的 IP 地址",
			headers: map[string]string{
				"X-Forwarded-For": "not-an-ip",
			},
			expectedIP:   "192.0.2.1",
			trustedProxy: true,
			description:  "无效的 IP 格式应该回退到 ClientIP",
		},
		{
			name: "空的 X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "",
				"X-Real-IP":       "",
			},
			expectedIP:   "192.0.2.1",
			trustedProxy: true,
			description:  "空的 header 应该回退到 ClientIP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trustedProxyCheck = tt.trustedProxy

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = &http.Request{
				Header: make(http.Header),
			}
			c.Request.RemoteAddr = "192.0.2.1:12345"

			for k, v := range tt.headers {
				c.Request.Header.Set(k, v)
			}

			ip := getClientIP(c)
			if ip != tt.expectedIP {
				t.Errorf("%s: 期望 IP %s, 得到 %s", tt.description, tt.expectedIP, ip)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalConf := config.Conf
	defer func() {
		config.Conf = originalConf
	}()

	config.Conf = &config.Config{
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 2,
			Burst:             3,
			BlockDuration:     time.Second * 2,
		},
	}

	mutex.Lock()
	ipLimiters = make(map[string]*ipLimiter)
	mutex.Unlock()

	middleware := RateLimit()
	handler := func(c *gin.Context) {
		middleware(c)
		c.String(http.StatusOK, "OK")
	}

	t.Run("允许正常请求通过", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			RemoteAddr: "10.0.0.1:12345",
			Header:     make(http.Header),
		}

		handler(c)

		if w.Code != http.StatusOK {
			t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
		}
	})

	t.Run("超过突发限制应该被拒绝", func(t *testing.T) {
		testIP := "10.0.0.2"

		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				RemoteAddr: testIP + ":12345",
				Header:     make(http.Header),
			}

			handler(c)

			if i < 3 {
				if w.Code != http.StatusOK {
					t.Errorf("请求 %d: 期望状态码 %d, 得到 %d", i+1, http.StatusOK, w.Code)
				}
			} else {
				if w.Code != http.StatusTooManyRequests {
					t.Errorf("请求 %d: 期望状态码 %d, 得到 %d", i+1, http.StatusTooManyRequests, w.Code)
				}
			}
		}
	})

	t.Run("阻塞期后应该恢复", func(t *testing.T) {
		testIP := "10.0.0.3"

		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = &http.Request{
				RemoteAddr: testIP + ":12345",
				Header:     make(http.Header),
			}
			handler(c)
		}

		time.Sleep(time.Second * 2)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			RemoteAddr: testIP + ":12345",
			Header:     make(http.Header),
		}
		handler(c)

		if w.Code != http.StatusOK {
			t.Errorf("阻塞期后请求应该成功，期望 %d, 得到 %d", http.StatusOK, w.Code)
		}
	})
}

func TestRateLimitDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalConf := config.Conf
	defer func() {
		config.Conf = originalConf
	}()

	config.Conf = &config.Config{
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 2,
			Burst:             3,
			BlockDuration:     time.Second * 5,
		},
	}

	mutex.Lock()
	ipLimiters = make(map[string]*ipLimiter)
	mutex.Unlock()

	middleware := RateLimit()
	handler := func(c *gin.Context) {
		middleware(c)
		c.String(http.StatusOK, "OK")
	}

	ip1 := "172.16.0.1"
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			RemoteAddr: ip1 + ":12345",
			Header:     make(http.Header),
		}
		handler(c)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		RemoteAddr: "172.16.0.2:12345",
		Header:     make(http.Header),
	}
	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("不同 IP 的限流应该是独立的，IP2 的请求应该成功")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalConf := config.Conf
	defer func() {
		config.Conf = originalConf
	}()

	config.Conf = &config.Config{
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             5,
			BlockDuration:     time.Second * 5,
		},
	}

	mutex.Lock()
	ipLimiters = make(map[string]*ipLimiter)
	mutex.Unlock()

	middleware := RateLimit()
	handler := func(c *gin.Context) {
		middleware(c)
		c.String(http.StatusOK, "OK")
	}

	testIP := "192.168.1.1"

	// 消耗所有令牌（burst=5）
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{
			RemoteAddr: testIP + ":12345",
			Header:     make(http.Header),
		}
		handler(c)
		if w.Code != http.StatusOK {
			t.Logf("请求 %d 被拒绝", i+1)
		}
	}

	time.Sleep(time.Millisecond * 500)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		RemoteAddr: testIP + ":12345",
		Header:     make(http.Header),
	}
	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("令牌桶应该在 0.5 秒后补充令牌，期望请求成功")
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"256.256.256.256", true},
		{"2001:db8::1", true},
		{"not-an-ip", false},
		{"", false},
		{"192.168.1", false},
	}

	for _, tt := range tests {
		result := isValidIP(tt.ip)
		if result != tt.expected {
			t.Errorf("IP %s: 期望 %v, 得到 %v", tt.ip, tt.expected, result)
		}
	}
}
