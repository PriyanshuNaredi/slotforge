package cache

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func InvalidateSlotSearch(ctx context.Context, client *redis.Client) {
	if client == nil {
		return
	}

	var cursor uint64

	for {
		keys, nextCursor, err := client.Scan(
			ctx,
			cursor,
			"slot-search:*",
			100,
		).Result()

		if err != nil {
			log.Printf("WARNING: failed to scan slot search cache keys: %v", err)
			return
		}

		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				log.Printf("WARNING: failed to invalidate slot search cache: %v", err)
				return
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			return
		}
	}
}
