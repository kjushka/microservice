package database

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kjushka/microservice-gen/internal/closer"
	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/kjushka/microservice-gen/internal/logger"
	"github.com/kjushka/microservice-gen/internal/storage"
	"github.com/kjushka/microservice-gen/internal/storage/serializer"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import for correct driver will be chosen
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
)

type DBStorage interface {
	storage.Storage
	GetDB() *sqlx.DB
}

func InitDB(ctx context.Context, cfg *config.Config, tracer trace.Tracer) (DBStorage, error) {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBPort,
		cfg.Database,
	)
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't connect with database")
	}

	err = db.Ping()
	if err != nil {
		for err != nil {
			time.Sleep(time.Second * 2)
			logger.InfoKV(ctx, "sleeping for wait db")
			err = db.Ping()
		}
	}

	closer.Add(db.Close)

	return &dbStorage{
		db,
		serializer.NewMessagePackSerializer(),
		tracer,
	}, nil
}

type dbStorage struct {
	db         *sqlx.DB
	serializer *serializer.MessagePackSerializer
	tracer     trace.Tracer
}

func (d *dbStorage) GetDB() *sqlx.DB {
	return d.db
}

func (d *dbStorage) Get(ctx context.Context, key string, table string, dest any) error {
	ctx, span := d.tracer.Start(ctx, "get from cache")
	defer span.End()

	var err error
	queryRow := d.db.QueryRowContext(ctx, strings.ReplaceAll(`
		select data from table where uid = $1;
	`, "table", table), key)
	if err = queryRow.Err(); err != nil {
		return fmt.Errorf("failed get from db: %s", err)
	}

	var data []byte
	err = queryRow.Scan(&data)

	if err != nil {
		return err
	}

	err = d.serializer.Decode(bytes.NewReader(data), dest)
	if err != nil {
		return fmt.Errorf("failed decode data: %v", err)
	}

	return nil
}

func (d *dbStorage) GetMany(ctx context.Context, keys []string, table string, dest ...any) error {
	ctx, span := d.tracer.Start(ctx, "get from cache")
	defer span.End()

	var err error
	queryBase := strings.ReplaceAll(`select data from table where uid in (?);`, "table", table)
	query, params, err := sqlx.In(queryBase, keys)
	if err != nil {
		return fmt.Errorf("failed prepare query: %v", err)
	}

	rows, err := d.db.QueryContext(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed get from db: %s", err)
	}

	var counter int
	for rows.Next() {
		var data []byte
		err = rows.Scan(&data)

		if err != nil {
			return err
		}

		err = d.serializer.Decode(bytes.NewReader(data), dest[counter])
		if err != nil {
			return fmt.Errorf("failed decode data: %v", err)
		}
	}

	return nil
}

func (d *dbStorage) Save(ctx context.Context, key string, data any, table string) error {
	ctx, span := d.tracer.Start(ctx, "save to cache")
	defer span.End()

	buf := bytes.NewBuffer(nil)
	err := d.serializer.Encode(buf, data)
	if err != nil {
		return fmt.Errorf("failed encode data: %v", err)
	}

	_, err = d.db.ExecContext(ctx, strings.ReplaceAll(`
		insert into table (uid, data)
		values ($1, $2) on conflict (uid) do
	update
	set data = excluded.data;
	`, "table", table), key)
	if err != nil {
		return fmt.Errorf("failed upsert data: %v", err)
	}

	return nil
}

func (d *dbStorage) Delete(ctx context.Context, key string, table string) error {
	ctx, span := d.tracer.Start(ctx, "save to cache")
	defer span.End()

	_, err := d.db.ExecContext(ctx, strings.ReplaceAll(`delete from table where uid = $1;`, "table", table), key)
	if err != nil {
		return fmt.Errorf("failed remove data from db: %v", err)
	}

	return nil
}
