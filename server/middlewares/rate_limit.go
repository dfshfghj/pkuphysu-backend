package middlewares

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
	blockEnd   time.Time
}

var (
	ipLimiters        = make(map[string]*ipLimiter)
	mutex             = sync.RWMutex{}
	trustedProxyCheck = true // 是否检查可信代理
)

func getClientIP(c *gin.Context) string {
	if trustedProxyCheck {
		hasProxyHeaders := c.GetHeader("X-Forwarded-For") != "" ||
			c.GetHeader("X-Real-IP") != "" ||
			c.GetHeader("Via") != ""

		if !hasProxyHeaders {
			return c.ClientIP()
		}
	}

	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if isValidIP(ip) {
				return ip
			}
		}
	}

	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if isValidIP(ip) {
			return ip
		}
	}

	return c.ClientIP()
}

func isValidIP(ip string) bool {
	if ip == "" {
		return false
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return true
	}
	if strings.Contains(ip, ":") {
		return true
	}
	return false
}

func cleanupExpired() {
	mutex.Lock()
	defer mutex.Unlock()

	now := time.Now()
	for ip, limiter := range ipLimiters {
		if limiter.blockEnd.Before(now) && now.Sub(limiter.lastAccess) > time.Hour {
			delete(ipLimiters, ip)
		}
	}
}

func RateLimit() func(c *gin.Context) {
	cfg := config.Conf

	requestsPerSecond := 10
	burst := 20
	blockDuration := time.Minute * 5

	if cfg.RateLimit.RequestsPerSecond > 0 {
		requestsPerSecond = cfg.RateLimit.RequestsPerSecond
	}
	if cfg.RateLimit.Burst > 0 {
		burst = cfg.RateLimit.Burst
	}
	if cfg.RateLimit.BlockDuration > 0 {
		blockDuration = cfg.RateLimit.BlockDuration
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			cleanupExpired()
		}
	}()

	return func(c *gin.Context) {
		ip := getClientIP(c)

		mutex.Lock()
		limiter, exists := ipLimiters[ip]
		if !exists {
			limiter = &ipLimiter{
				limiter:    rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
				lastAccess: time.Now(),
			}
			ipLimiters[ip] = limiter
		}
		limiter.lastAccess = time.Now()
		mutex.Unlock()

		if !limiter.blockEnd.IsZero() && time.Now().Before(limiter.blockEnd) {
			utils.RespondError(c, http.StatusTooManyRequests, "Too Many Requests", fmt.Errorf("rate limit exceeded"))
			c.Abort()
			return
		}

		if !limiter.blockEnd.IsZero() && time.Now().After(limiter.blockEnd) {
			limiter.blockEnd = time.Time{}
		}

		if !limiter.limiter.Allow() {
			limiter.blockEnd = time.Now().Add(blockDuration)
			utils.RespondError(c, http.StatusTooManyRequests, "Too Many Requests", fmt.Errorf("rate limit exceeded"))
			c.Abort()
			return
		}

		c.Next()
	}
}
