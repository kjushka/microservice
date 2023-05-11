package sharding

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-pg/sharding/v8"
	"hash/fnv"
	"io"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/kjushka/microservice-gen/internal/closer"
	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/kjushka/microservice-gen/internal/logger"
	"github.com/kjushka/microservice-gen/internal/storage"
	"github.com/kjushka/microservice-gen/internal/storage/serializer"
	"go.opentelemetry.io/otel/trace"
)

type ClusterStorage interface {
	storage.Storage
	GetCluster() *sharding.Cluster
}

func InitDB(ctx context.Context, cfg *config.Config, tracer trace.Tracer) (ClusterStorage, error) {
	db := pg.Connect(&pg.Options{
		User:     cfg.DBUser,
		Password: cfg.DBPass,
		Addr:     fmt.Sprintf("%s:%s", cfg.DBHost, cfg.DBPort),
	})

	err := db.Ping(ctx)
	if err != nil {
		for err != nil {
			time.Sleep(time.Second * 2)
			logger.InfoKV(ctx, "sleeping for wait db")
			err = db.Ping(ctx)
		}
	}

	dbs := []*pg.DB{db} // list of physical PostgreSQL servers
	cluster := sharding.NewCluster(dbs, cfg.DBShardsCount)
	closer.Add(cluster.Close)

	shardByKeyFn := func(shardsCount int) func(key string) int64 {
		return func(key string) int64 {
			hf := fnv.New32a()
			_, _ = io.WriteString(hf, key)
			return int64(hf.Sum32() % uint32(shardsCount))
		}
	}(cfg.DBShardsCount)

	return &clusterStorage{
		cluster,
		shardByKeyFn,
		serializer.NewMessagePackSerializer(),
		tracer,
	}, nil
}

type clusterStorage struct {
	cluster    *sharding.Cluster
	shardByKey func(key string) int64
	serializer *serializer.MessagePackSerializer
	tracer     trace.Tracer
}

func (d *clusterStorage) GetCluster() *sharding.Cluster {
	return d.cluster
}

func (d *clusterStorage) Get(ctx context.Context, key string, table string, dest any) error {
	ctx, span := d.tracer.Start(ctx, "get from cache")
	defer span.End()

	var data []byte
	_, err := d.cluster.Shard(d.shardByKey(key)).QueryOneContext(ctx, pg.Scan(data), strings.ReplaceAll(`
		select data from ?SHARD.table where uid = $1;
	`, "table", table), key)
	if err != nil {
		return fmt.Errorf("failed get from db: %s", err)
	}

	err = d.serializer.Decode(bytes.NewReader(data), dest)
	if err != nil {
		return fmt.Errorf("failed decode data: %v", err)
	}

	return nil
}

func (d *clusterStorage) GetMany(ctx context.Context, keys []string, table string, dest ...any) error {
	panic("not implemented")
}

func (d *clusterStorage) Save(ctx context.Context, key string, data any, table string) error {
	ctx, span := d.tracer.Start(ctx, "save to cache")
	defer span.End()

	buf := bytes.NewBuffer(nil)
	err := d.serializer.Encode(buf, data)
	if err != nil {
		return fmt.Errorf("failed encode data: %v", err)
	}

	_, err = d.cluster.Shard(d.shardByKey(key)).ExecContext(ctx, strings.ReplaceAll(`
		insert into ?SHARD.table (uid, data)
		values ($1, $2) on conflict (uid) do
		update
		set data = excluded.data;
	`, "table", table), key)
	if err != nil {
		return fmt.Errorf("failed upsert data: %v", err)
	}

	return nil
}

func (d *clusterStorage) Delete(ctx context.Context, key string, table string) error {
	ctx, span := d.tracer.Start(ctx, "save to cache")
	defer span.End()

	_, err := d.cluster.Shard(d.shardByKey(key)).ExecContext(ctx, strings.ReplaceAll(`delete from ?SHADR.table where uid = $1;`, "table", table), key)
	if err != nil {
		return fmt.Errorf("failed remove data from db: %v", err)
	}

	return nil
}
