package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

//go:embed lua/rate_limit.lua
var rateLimitScript string

type RedisLimiter struct {
	client         *redis.Client
	limitScriptSHA string
}

var (
	instance *RedisLimiter
	once     sync.Once
)

func NewRedisLimiter(ctx context.Context, r *redis.Client) *RedisLimiter {
	once.Do(func() {
		limitSHA, err := r.ScriptLoad(ctx, rateLimitScript).Result()
		if err != nil {
			platformobservability.SysLog(fmt.Sprintf("Failed to load rate limit script: %v", err))
		}
		instance = &RedisLimiter{
			client:         r,
			limitScriptSHA: limitSHA,
		}
	})

	return instance
}

func (rl *RedisLimiter) Allow(ctx context.Context, key string, opts ...Option) (bool, error) {
	config := &Config{
		Capacity:  10,
		Rate:      1,
		Requested: 1,
	}

	for _, opt := range opts {
		opt(config)
	}

	result, err := rl.client.EvalSha(
		ctx,
		rl.limitScriptSHA,
		[]string{key},
		config.Requested,
		config.Rate,
		config.Capacity,
	).Int()
	if err != nil {
		return false, fmt.Errorf("rate limit failed: %w", err)
	}

	return result == 1, nil
}

type Config struct {
	Capacity  int64
	Rate      float64
	Requested int64
}

type Option func(*Config)

func WithCapacity(c int64) Option {
	return func(cfg *Config) { cfg.Capacity = c }
}

func WithRate(r float64) Option {
	return func(cfg *Config) { cfg.Rate = r }
}

func WithRequested(n int64) Option {
	return func(cfg *Config) { cfg.Requested = n }
}
