package cache

import (
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient accepts either a host:port address or a redis:// URL —
// deployment platforms (Render Key Value) hand out the URL form.
func NewRedisClient(addr string) *redis.Client {
	options := &redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolTimeout:  2 * time.Second,
		MaxRetries:   1,
	}

	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		if parsed, err := redis.ParseURL(addr); err == nil {
			parsed.DialTimeout = options.DialTimeout
			parsed.ReadTimeout = options.ReadTimeout
			parsed.WriteTimeout = options.WriteTimeout
			parsed.PoolTimeout = options.PoolTimeout
			parsed.MaxRetries = options.MaxRetries
			options = parsed
		}
	}

	return redis.NewClient(options)
}
