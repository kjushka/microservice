package cache

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kjushka/microservice-gen/internal/closer"
	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/kjushka/microservice-gen/internal/storage"
	"github.com/kjushka/microservice-gen/internal/storage/serializer"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

type Cache interface {
	storage.Storage

	RedisClient() *redis.Client
}

func InitCache(cfg *config.Config, tracer trace.Tracer) (Cache, error) {
	rdb := &cache{
		redisClient: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("redis:%s", cfg.CachePort),
			Password: "",
			DB:       0,
		}),
		serializer: serializer.NewMessagePackSerializer(),
		expireTime: cfg.CacheExpirationTime,
		tracer:     tracer,
	}

	closer.Add(rdb.redisClient.Close)

	_, err := rdb.redisClient.Ping(context.Background()).Result()
	if err != nil {
		return nil, errors.Wrap(err, "error in ping redis")
	}

	return rdb, nil
}

type cache struct {
	redisClient *redis.Client
	serializer  *serializer.MessagePackSerializer
	expireTime  time.Duration
	tracer      trace.Tracer
}

func (c *cache) RedisClient() *redis.Client {
	return c.redisClient
}

func (c *cache) Get(ctx context.Context, key string, table string, dest any) error {
	ctx, span := c.tracer.Start(ctx, "get from db")
	defer span.End()

	key = fmt.Sprintf("%s-%s", key, table)
	encoded, err := c.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return fmt.Errorf("failed get from redis: %v", err)
	}

	err = c.serializer.Decode(bytes.NewReader(encoded), dest)
	if err != nil {
		return fmt.Errorf("failed decoding: %v", err)
	}

	return nil
}

func (c *cache) GetMany(ctx context.Context, keys []string, table string, dest ...any) error {
	ctx, span := c.tracer.Start(ctx, "get many from db")
	defer span.End()

	if len(keys) != len(dest) {
		return errors.New("len of keys not equal len of dest")
	}

	var err error
	for i, key := range keys {
		err = c.Get(ctx, key, table, dest[i])
		if err != nil {
			return fmt.Errorf("failed get item with key '%s': %v", key, err)
		}
	}

	return nil
}

func (c *cache) Save(ctx context.Context, key string, data any, table string) error {
	ctx, span := c.tracer.Start(ctx, "save to db")
	defer span.End()

	key = fmt.Sprintf("%s-%s", key, table)

	buf := bytes.NewBuffer(nil)
	err := c.serializer.Encode(buf, data)
	if err != nil {
		return fmt.Errorf("failed encode data: %v", err)
	}

	err = c.redisClient.Set(ctx, key, buf.Bytes(), c.expireTime).Err()
	if err != nil {
		return fmt.Errorf("failed set data to redis: %v", err)
	}

	return nil
}

func (c *cache) Delete(ctx context.Context, key string, table string) error {
	ctx, span := c.tracer.Start(ctx, "delete in db")
	defer span.End()

	key = fmt.Sprintf("%s-%s", key, table)
	err := c.redisClient.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed remove from redis: %v", err)
	}

	return nil
}
