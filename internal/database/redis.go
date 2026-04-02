package database

import (
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
)

type Redis struct {
	rd *redis.Client
}

func NewRedis() *Redis {
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("Redis") + ":" + os.Getenv("RedisPort"),
		Password: "",
		DB:       0,
	})

	log.Println("Redis configurated")

	return &Redis{client}
}

func (r *Redis) SetData(item string, data interface{}, expTime time.Duration) error {
	if err := r.rd.Set(item, data, expTime).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) GetData(item string) (string, error) {
	data, err := r.rd.Get(item).Result()
	if err != nil {
		return "", err
	}
	return data, nil
}

func (r *Redis) Make(a ...interface{}) {
	r.rd.Do(a...).Result()
}

func (r *Redis) IterateWithPattern(pattern string) ([]string, error) {
	var keys []string
	var cursor uint64

	for {
		var batch []string
		var err error

		batch, cursor, err = r.rd.Scan(cursor, pattern, 20).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, batch...)

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}
