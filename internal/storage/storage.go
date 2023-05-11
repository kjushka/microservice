package storage

import (
	"context"
)

type Storage interface {
	// need send pointer to dest
	Get(ctx context.Context, key string, table string, dest any) error
	// need send pointer to dest
	GetMany(ctx context.Context, keys []string, table string, dest ...any) error
	Save(ctx context.Context, key string, data any, table string) error
	Delete(ctx context.Context, key string, table string) error
}
