package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiterClient struct {
	timestamps []time.Time
}

const numShards = 16

type limiterShard struct {
	sync.Mutex
	clients map[string]*rateLimiterClient
}

// RateLimiter implements a sliding-window rate limiter setting X-RateLimit headers.
func RateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	return RateLimiterWithContext(context.Background(), maxRequests, window)
}

// RateLimiterWithContext implements a sliding-window rate limiter with context cancellation for clean shutdown.
func RateLimiterWithContext(ctx context.Context, maxRequests int, window time.Duration) gin.HandlerFunc {
	shards := make([]*limiterShard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &limiterShard{
			clients: make(map[string]*rateLimiterClient),
		}
	}

	getShard := func(key string) *limiterShard {
		var h uint32 = 2166136261
		for i := 0; i < len(key); i++ {
			h ^= uint32(key[i])
			h *= 16777619
		}
		return shards[h%numShards]
	}

	go func() {
		ticker := time.NewTicker(window * 2)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				cutoff := now.Add(-window)
				for _, sh := range shards {
					sh.Lock()
					for id, client := range sh.clients {
						valid := client.timestamps[:0]
						for _, ts := range client.timestamps {
							if ts.After(cutoff) {
								valid = append(valid, ts)
							}
						}
						if len(valid) == 0 {
							delete(sh.clients, id)
						} else {
							client.timestamps = valid
						}
					}
					sh.Unlock()
				}
			}
		}
	}()

	return func(c *gin.Context) {
		// Anchor by ClientIP to prevent unauthenticated spoofed X-API-Key from bypassing IP limits
		clientKey := "ip:" + c.ClientIP()
		if pid := GetProjectID(c); pid != "" && pid != "default" {
			clientKey = "proj:" + pid
		}

		now := time.Now()
		cutoff := now.Add(-window)

		sh := getShard(clientKey)
		sh.Lock()
		client, exists := sh.clients[clientKey]
		if !exists {
			client = &rateLimiterClient{timestamps: make([]time.Time, 0, maxRequests)}
			sh.clients[clientKey] = client
		}

		validTimestamps := client.timestamps[:0]
		for _, ts := range client.timestamps {
			if ts.After(cutoff) {
				validTimestamps = append(validTimestamps, ts)
			}
		}
		client.timestamps = validTimestamps

		currentCount := len(client.timestamps)
		remaining := maxRequests - currentCount
		if remaining < 0 {
			remaining = 0
		}

		resetSeconds := int(window.Seconds())
		if currentCount > 0 {
			oldest := client.timestamps[0]
			resetDuration := oldest.Add(window).Sub(now)
			if resetDuration > 0 {
				resetSeconds = int(resetDuration.Seconds()) + 1
			}
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resetSeconds))

		if currentCount >= maxRequests {
			sh.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": fmt.Sprintf("Rate limit exceeded. Try again in %d seconds.", resetSeconds),
			})
			c.Abort()
			return
		}

		client.timestamps = append(client.timestamps, now)
		sh.Unlock()

		c.Next()
	}
}
