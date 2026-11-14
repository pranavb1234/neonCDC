package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func Connect(ctx context.Context, sourceDBURL string) (*pgx.Conn, error) {
	connConfig, err := pgx.ParseConfig(sourceDBURL)
	if err != nil {
		return nil, errors.New("could not parse PostgreSQL connection settings")
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, errors.New("could not connect to PostgreSQL")
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close(context.Background())
		return nil, errors.New("could not ping PostgreSQL")
	}

	return conn, nil
}
