package database

import (
	"context"
	"os"
	"time"

	golog "github.com/Vladroon22/GoLog"
	pool "github.com/jackc/pgx/v5/pgxpool"
)

type DataBase struct {
	logger *golog.Logger
}

func NewDB(logg *golog.Logger) *DataBase {
	return &DataBase{
		logger: logg,
	}
}

func (d *DataBase) Connect(ctx context.Context) (*pool.Pool, error) {
	dbURL := os.Getenv("DB")
	if dbURL == "" {
		d.logger.Fatalln("dbURL doesn't set")
	}

	pool, err := pool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	if err := ping(ctx, pool); err != nil {
		return nil, err
	}
	d.logger.Infoln("Database connection is valid")
	return pool, nil
}

func ping(c context.Context, cn *pool.Pool) error {
	ctx, cancel := context.WithTimeout(c, time.Second*3)
	defer cancel()
	var err error
	for i := 0; i < 5; i++ {
		if err = cn.Ping(ctx); err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	return err
}
