package health

import (
	"github.com/redis/go-redis/v9"
)

// newTestRedisClient builds a Redis client accepting either host:port or a
// redis:// URL, mirroring cache.NewRedisClient without importing the cache
// package (and its invalidation side effects) into health tests.
func newTestRedisClient(addr string) *redis.Client {
	if parsed, err := redis.ParseURL(addr); err == nil {
		return redis.NewClient(parsed)
	}
	return redis.NewClient(&redis.Options{Addr: addr})
}
