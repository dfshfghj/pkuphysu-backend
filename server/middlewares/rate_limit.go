package middlewares

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	lastAccess time.Time
	count      int
	blocked    bool
	blockEnd   time.Time
}

var (
	ipLimiters = make(map[string]*rateLimiter)
	mutex      = sync.RWMutex{}
)

func getClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	return c.ClientIP()
}

// cleanupExpired 清理过期的IP记录（简单实现，实际生产环境可能需要更高效的方案）
func cleanupExpired() {
	mutex.Lock()
	defer mutex.Unlock()

	now := time.Now()
	for ip, limiter := range ipLimiters {
		if !limiter.blocked && now.Sub(limiter.lastAccess) > time.Hour {
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

	return func(c *gin.Context) {
		ip := getClientIP(c)

		mutex.Lock()
		limiter, exists := ipLimiters[ip]
		if !exists {
			limiter = &rateLimiter{
				lastAccess: time.Now(),
				count:      0,
			}
			ipLimiters[ip] = limiter
		}

		now := time.Now()

		if limiter.blocked && now.Before(limiter.blockEnd) {
			mutex.Unlock()
			utils.RespondError(c, http.StatusTooManyRequests, "Too Many Requests", nil)
			c.Abort()
			return
		}

		if limiter.blocked && now.After(limiter.blockEnd) {
			limiter.blocked = false
			limiter.count = 0
		}

		window := time.Duration(1000000000/requestsPerSecond) * time.Nanosecond
		if now.Sub(limiter.lastAccess) > window {
			limiter.count = 1
		} else {
			limiter.count++
		}

		limiter.lastAccess = now

		if limiter.count > burst {
			limiter.blocked = true
			limiter.blockEnd = now.Add(blockDuration)
			mutex.Unlock()
			utils.RespondError(c, http.StatusTooManyRequests, "Too Many Requests", nil)
			c.Abort()
			return
		}

		mutex.Unlock()

		if len(ipLimiters)%1000 == 0 {
			cleanupExpired()
		}

		c.Next()
	}
}
