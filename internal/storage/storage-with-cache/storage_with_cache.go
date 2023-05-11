package storage_with_cache

import (
	"context"
	"github.com/kjushka/microservice-gen/internal/errgroup"
	"github.com/kjushka/microservice-gen/internal/storage"
	"github.com/kjushka/microservice-gen/internal/storage/cache"
	"github.com/kjushka/microservice-gen/internal/storage/database"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/trace"
)

func NewStorage(cache cache.Cache, db database.DBStorage, tracer trace.Tracer) storage.Storage {
	return &storageWithCache{
		cache:  cache,
		db:     db,
		tracer: tracer,
	}
}

type storageWithCache struct {
	cache  cache.Cache
	db     database.DBStorage
	tracer trace.Tracer
}

func (s *storageWithCache) Get(ctx context.Context, key string, table string, dest any) error {
	var err error
	err = s.cache.Get(ctx, key, table, dest)
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		return nil
	}
	err = s.db.Get(ctx, key, table, dest)
	if err != nil {
		return err
	}
	return nil
}

func (s *storageWithCache) GetMany(ctx context.Context, keys []string, table string, dest ...any) error {
	var err error
	err = s.cache.GetMany(ctx, keys, table, dest)
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		return nil
	}
	err = s.db.GetMany(ctx, keys, table, dest)
	if err != nil {
		return err
	}
	return nil
}

func (s *storageWithCache) Save(ctx context.Context, key string, data any, table string) error {
	g, errCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		mErr := s.cache.Save(errCtx, key, data, table)
		if mErr != nil {
			return mErr
		}
		return nil
	})
	g.Go(func() error {
		mErr := s.cache.Save(errCtx, key, data, table)
		if mErr != nil {
			return mErr
		}
		return nil
	})
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}

func (s *storageWithCache) Delete(ctx context.Context, key string, table string) error {
	g, errCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		mErr := s.cache.Delete(errCtx, key, table)
		if mErr != nil {
			return mErr
		}
		return nil
	})
	g.Go(func() error {
		mErr := s.cache.Delete(errCtx, key, table)
		if mErr != nil {
			return mErr
		}
		return nil
	})
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}
