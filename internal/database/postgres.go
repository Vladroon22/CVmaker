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

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(20),
		hash_password VARCHAR(70) NOT NULL,
		email VARCHAR(30) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP 
	);
	
	CREATE TABLE IF NOT EXISTS sessions (
		id SERIAL PRIMARY KEY,
		user_id INT REFERENCES users(id) NOT NULL, 
		device_type VARCHAR(15) NOT NULL,
		created_at TIMESTAMP   
	)
	`

	if _, err := pool.Exec(ctx, schema); err != nil {
		panic("Error creating migrations:" + err.Error())
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
