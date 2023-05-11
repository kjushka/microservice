package migrator

import (
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // import for reading migrations file
	"github.com/jmoiron/sqlx"
	"github.com/kjushka/microservice-gen/internal/config"
	"github.com/pkg/errors"
)

func Migrate(db *sqlx.DB, cfg *config.Config) error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{
		DatabaseName: cfg.Database,
	})
	if err != nil {
		return errors.Wrap(err, "error to define driver")
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres", driver,
	)
	if err != nil {
		return errors.Wrap(err, "error in create migration client")
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		downErr := m.Down()
		if downErr != nil {
			return errors.Wrap(err, "error in up and down migration")
		}
		return errors.Wrap(err, "error in up migration")
	}

	return nil
}
