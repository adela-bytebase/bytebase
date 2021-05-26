package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

var (
	_ Driver = (*MySQLDriver)(nil)
)

func init() {
	register(Mysql, newDriver)
}

type MySQLDriver struct {
	l *zap.Logger

	db *sql.DB
}

func newDriver(config DriverConfig) Driver {
	return &MySQLDriver{
		l: config.Logger,
	}
}

func (driver *MySQLDriver) open(config ConnectionConfig) (Driver, error) {
	protocol := "tcp"
	if strings.HasPrefix(config.Host, "/") {
		protocol = "unix"
	}

	dsn := fmt.Sprintf("%s:%s@%s(%s:%s)/%s", config.Username, config.Password, protocol, config.Host, config.Port, config.Database)
	driver.l.Debug("Opening MySQL driver", zap.String("dsn", dsn))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	driver.db = db

	return driver, nil
}

func (driver *MySQLDriver) Ping(ctx context.Context) error {
	return driver.db.PingContext(ctx)
}

func (driver *MySQLDriver) SyncSchema(ctx context.Context) ([]*DBSchema, error) {
	excludedDatabaseList := []string{
		"'mysql'",
		"'information_schema'",
		"'performance_schema'",
		"'sys'",
	}

	where := fmt.Sprintf("SCHEMA_NAME NOT IN (%s)", strings.Join(excludedDatabaseList, ", "))

	rows, err := driver.db.QueryContext(ctx, `
		SELECT 
		    SCHEMA_NAME
		FROM information_schema.SCHEMATA
		WHERE `+where,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*DBSchema, 0)
	for rows.Next() {
		var schema DBSchema
		if err := rows.Scan(
			&schema.Name,
		); err != nil {
			return nil, err
		}

		list = append(list, &schema)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return list, err
}

func (driver *MySQLDriver) Execute(ctx context.Context, sql string) (sql.Result, error) {
	tx, err := driver.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.ExecContext(ctx, sql)
}