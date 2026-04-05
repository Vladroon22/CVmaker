package database

import (
	"context"
	"log"
	"os"
	"time"

	pool "github.com/jackc/pgx/v5/pgxpool"
)

type DataBase struct {
	pool *pool.Pool
}

func NewDB() *DataBase {
	return &DataBase{}
}

func (d *DataBase) GetPool() *pool.Pool {
	return d.pool
}

func (d *DataBase) Connect(ctx context.Context) error {
	dbURL := os.Getenv("DB")
	if dbURL == "" {
		log.Fatalln("dbURL doesn't set")
	}

	pool, err := pool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	if err := ping(ctx, pool); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(20),
		hash_password VARCHAR(70) NOT NULL,
		email VARCHAR(30) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP 
	);
	
	CREATE TABLE IF NOT EXISTS sessions (
		id SERIAL PRIMARY KEY,
		user_id UUID REFERENCES users(id) NOT NULL, 
		device_type VARCHAR(15) NOT NULL,
		created_at TIMESTAMP   
	)
	`

	if _, err := pool.Exec(ctx, schema); err != nil {
		panic("Error creating migrations:" + err.Error())
	}
	d.pool = pool

	log.Println("Database connection is valid")
	return nil
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

func (d *DataBase) Close() {
	d.pool.Close()
}
