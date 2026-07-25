package cache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

var RDB *redis.Client
var RedisEnabled = true

type RedisRuntimeConfig struct {
	DebugEnabled  bool
	SyncFrequency int
	Logf          func(string)
	FatalLog      func(string)
}

var redisRuntimeConfig = RedisRuntimeConfig{
	DebugEnabled:  false,
	SyncFrequency: 60,
	Logf: func(message string) {
		log.Println(message)
	},
	FatalLog: func(message string) {
		log.Fatal(message)
	},
}

func ConfigureRedisRuntime(config RedisRuntimeConfig) {
	if config.SyncFrequency > 0 {
		redisRuntimeConfig.SyncFrequency = config.SyncFrequency
	}
	redisRuntimeConfig.DebugEnabled = config.DebugEnabled
	if config.Logf != nil {
		redisRuntimeConfig.Logf = config.Logf
	}
	if config.FatalLog != nil {
		redisRuntimeConfig.FatalLog = config.FatalLog
	}
}

// RedisReady reports whether the shared Redis client is available.
func RedisReady() bool {
	return RedisEnabled && RDB != nil
}

// RedisKeyCacheSeconds returns the cache TTL used for Redis-backed entity caches.
func RedisKeyCacheSeconds() int {
	return redisRuntimeConfig.SyncFrequency
}

// InitRedisClient initializes the shared Redis client from environment configuration.
func InitRedisClient() error {
	if os.Getenv("REDIS_CONN_STRING") == "" {
		RedisEnabled = false
		redisRuntimeConfig.Logf("REDIS_CONN_STRING not set, Redis is not enabled")
		return nil
	}
	if os.Getenv("SYNC_FREQUENCY") == "" {
		redisRuntimeConfig.Logf("SYNC_FREQUENCY not set, use default value 60")
		redisRuntimeConfig.SyncFrequency = 60
	}
	redisRuntimeConfig.Logf("Redis is enabled")
	opt, err := redis.ParseURL(os.Getenv("REDIS_CONN_STRING"))
	if err != nil {
		redisRuntimeConfig.FatalLog("failed to parse Redis connection string: " + err.Error())
	}
	opt.PoolSize = getEnvOrDefaultInt("REDIS_POOL_SIZE", 10)
	RDB = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err = RDB.Ping(ctx).Result(); err != nil {
		redisRuntimeConfig.FatalLog("Redis ping test failed: " + err.Error())
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis connected to %s", opt.Addr))
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis database: %d", opt.DB))
	}
	return nil
}

// ParseRedisOption parses REDIS_CONN_STRING into redis.Options.
func ParseRedisOption() *redis.Options {
	opt, err := redis.ParseURL(os.Getenv("REDIS_CONN_STRING"))
	if err != nil {
		redisRuntimeConfig.FatalLog("failed to parse Redis connection string: " + err.Error())
	}
	return opt
}

func RedisSet(key string, value string, expiration time.Duration) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis SET: key=%s, value=%s, expiration=%v", key, value, expiration))
	}
	return RDB.Set(context.Background(), key, value, expiration).Err()
}

func RedisGet(key string) (string, error) {
	if !RedisReady() {
		return "", fmt.Errorf("redis client is not initialized")
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis GET: key=%s", key))
	}
	return RDB.Get(context.Background(), key).Result()
}

func RedisDel(key string) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis DEL: key=%s", key))
	}
	return RDB.Del(context.Background(), key).Err()
}

func RedisDelKey(key string) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis DEL Key: key=%s", key))
	}
	return RDB.Del(context.Background(), key).Err()
}

func RedisHSetObj(key string, obj interface{}, expiration time.Duration) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis HSET: key=%s, obj=%+v, expiration=%v", key, obj, expiration))
	}

	ctx := context.Background()
	data := make(map[string]interface{})
	v := reflect.ValueOf(obj).Elem()
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if field.Type.String() == "gorm.DeletedAt" {
			continue
		}
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				data[field.Name] = ""
				continue
			}
			value = value.Elem()
		}
		if value.Kind() == reflect.Bool {
			data[field.Name] = strconv.FormatBool(value.Bool())
			continue
		}
		data[field.Name] = fmt.Sprintf("%v", value.Interface())
	}

	txn := RDB.TxPipeline()
	txn.HSet(ctx, key, data)
	if expiration > 0 {
		txn.Expire(ctx, key, expiration)
	}
	if _, err := txn.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}
	return nil
}

func RedisHGetObj(key string, obj interface{}) error {
	if !RedisReady() {
		return fmt.Errorf("redis client is not initialized")
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis HGETALL: key=%s", key))
	}

	result, err := RDB.HGetAll(context.Background(), key).Result()
	if err != nil {
		return fmt.Errorf("failed to load hash from Redis: %w", err)
	}
	if len(result) == 0 {
		return fmt.Errorf("key %s not found in Redis", key)
	}

	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("obj must be a pointer to a struct, got %T", obj)
	}
	v := val.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("obj must be a pointer to a struct, got pointer to %T", v.Interface())
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldName := field.Name
		value, ok := result[fieldName]
		if !ok {
			continue
		}
		fieldValue := v.Field(i)
		if fieldValue.Kind() == reflect.Ptr {
			if value == "" {
				continue
			}
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
			}
			fieldValue = fieldValue.Elem()
		}

		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(value)
		case reflect.Int, reflect.Int64:
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse int field %s: %w", fieldName, err)
			}
			fieldValue.SetInt(intValue)
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("failed to parse bool field %s: %w", fieldName, err)
			}
			fieldValue.SetBool(boolValue)
		case reflect.Struct:
			if fieldValue.Type().String() == "gorm.DeletedAt" && value != "" {
				timeValue, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return fmt.Errorf("failed to parse DeletedAt field %s: %w", fieldName, err)
				}
				fieldValue.Set(reflect.ValueOf(gorm.DeletedAt{Time: timeValue, Valid: true}))
			}
		default:
			return fmt.Errorf("unsupported field type: %s for field %s", fieldValue.Kind(), fieldName)
		}
	}
	return nil
}

func RedisIncr(key string, delta int64) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis INCR: key=%s, delta=%d", key, delta))
	}
	ttl, err := RDB.TTL(context.Background(), key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl <= 0 {
		return nil
	}

	ctx := context.Background()
	txn := RDB.TxPipeline()
	if err := txn.IncrBy(ctx, key, delta).Err(); err != nil {
		return err
	}
	txn.Expire(ctx, key, ttl)
	_, err = txn.Exec(ctx)
	return err
}

func RedisHIncrBy(key string, field string, delta int64) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis HINCRBY: key=%s, field=%s, delta=%d", key, field, delta))
	}
	ttl, err := RDB.TTL(context.Background(), key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl <= 0 {
		return nil
	}

	ctx := context.Background()
	txn := RDB.TxPipeline()
	if err := txn.HIncrBy(ctx, key, field, delta).Err(); err != nil {
		return err
	}
	txn.Expire(ctx, key, ttl)
	_, err = txn.Exec(ctx)
	return err
}

func RedisHSetField(key string, field string, value interface{}) error {
	if !RedisReady() {
		return nil
	}
	if redisRuntimeConfig.DebugEnabled {
		redisRuntimeConfig.Logf(fmt.Sprintf("Redis HSET field: key=%s, field=%s, value=%v", key, field, value))
	}
	ttl, err := RDB.TTL(context.Background(), key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("failed to get TTL: %w", err)
	}
	if ttl <= 0 {
		return nil
	}

	ctx := context.Background()
	txn := RDB.TxPipeline()
	if err := txn.HSet(ctx, key, field, value).Err(); err != nil {
		return err
	}
	txn.Expire(ctx, key, ttl)
	_, err = txn.Exec(ctx)
	return err
}

func getEnvOrDefaultInt(env string, defaultValue int) int {
	envValue := os.Getenv(env)
	if envValue == "" {
		return defaultValue
	}
	num, err := strconv.Atoi(envValue)
	if err != nil {
		return defaultValue
	}
	return num
}
