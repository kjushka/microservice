package config

import (
	"context"
	"errors"
	"fmt"
	"github.com/kjushka/microservice-gen/internal/logger"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DBHost, DBPort, Database, DBUser, DBPass string
	DBTimeout                                time.Duration
	DBShardsCount                            int
	CachePort                                string
	CacheTimeout                             time.Duration
	CacheExpirationTime                      time.Duration
	RateLimiterCapacity                      int64
}

func InitConfig(ctx context.Context) (*Config, error) {
	pgHost, ok := os.LookupEnv("PG_HOST")
	if !ok {
		return nil, errors.New("PG_ADDR not found")
	}
	pgPort, ok := os.LookupEnv("PG_PORT")
	if !ok {
		return nil, errors.New("PG_PORT not found")
	}
	database, ok := os.LookupEnv("PG_DATABASE")
	if !ok {
		return nil, errors.New("PG_WALLET_DATABASE not found")
	}
	pgUser, ok := os.LookupEnv("PG_USER")
	if !ok {
		return nil, errors.New("PG_USER not found")
	}
	pgPass, ok := os.LookupEnv("PG_PASS")
	if !ok {
		return nil, errors.New("PG_PASS not found")
	}
	pgTimeoutStr, ok := os.LookupEnv("PG_TIMEOUT")
	if !ok {
		return nil, errors.New("PG_TIMEOUT not found")
	}
	pgTimeout, err := time.ParseDuration(pgTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("failed parse pgsql timeout: %v", err)
	}
	pgShardsStr, ok := os.LookupEnv("PG_SHARDS_COUNT")
	if !ok {
		return nil, errors.New("PG_SHARDS_COUNT not found")
	}
	pgShards, err := strconv.Atoi(pgShardsStr)
	if err != nil {
		return nil, fmt.Errorf("failed parse pgsql shards count: %v", err)
	}

	redisPort, ok := os.LookupEnv("REDIS_PORT")
	if !ok {
		return nil, errors.New("REDIS_PORT not found")
	}
	redisTimeoutStr, ok := os.LookupEnv("REDIS_TIMEOUT")
	if !ok {
		return nil, errors.New("REDIS_TIMEOUT not found")
	}
	redisTimeout, err := time.ParseDuration(redisTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("failed parse redis timeout: %v", err)
	}
	redisExpirationTimeStr, ok := os.LookupEnv("REDIS_EXPIRATION_TIME")
	if !ok {
		return nil, errors.New("REDIS_EXPIRATION_TIME not found")
	}
	redisExpirationTime, err := time.ParseDuration(redisExpirationTimeStr)
	if err != nil {
		return nil, fmt.Errorf("failed parse redis expiration time: %v", err)
	}

	rateLimiterCapacityStr, ok := os.LookupEnv("RATE_LIMITER_CAPACITY")
	if !ok {
		return nil, errors.New("RATE_LIMITER_CAPACITY not found")
	}
	rateLimiterCapacity, err := strconv.ParseInt(rateLimiterCapacityStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed parse rate limiter capacity: %v", err)
	}

	config := &Config{
		DBHost:              pgHost,
		DBPort:              pgPort,
		DBUser:              pgUser,
		DBPass:              pgPass,
		Database:            database,
		DBTimeout:           pgTimeout,
		DBShardsCount:       pgShards,
		CachePort:           redisPort,
		CacheTimeout:        redisTimeout,
		CacheExpirationTime: redisExpirationTime,
		RateLimiterCapacity: rateLimiterCapacity,
	}

	logger.InfoKV(ctx, "config initialized", "config", config)

	return config, nil
}
