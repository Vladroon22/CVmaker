package database

import (
	"context"
	"time"

	"github.com/Vladroon22/CVmaker/config"
	golog "github.com/Vladroon22/GoLog"
	pool "github.com/jackc/pgx/v5/pgxpool"
)

type DataBase struct {
	logger *golog.Logger
	config *config.Config
	sql    *pool.Pool
}

func NewDB(conf *config.Config, logg *golog.Logger) *DataBase {
	return &DataBase{
		config: conf,
		logger: logg,
	}
}

func (d *DataBase) Connect(ctx context.Context) (*pool.Pool, error) {
	pool, err := pool.New(ctx, "postgres://"+d.config.DB)
	if err != nil {
		return nil, err
	}
	if err := ping(ctx, pool); err != nil {
		return nil, err
	}
	d.logger.Infoln("Database connection is valid")
	d.sql = pool
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
