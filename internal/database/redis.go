package database

import (
	"time"

	"github.com/Vladroon22/CVmaker/config"
	golog "github.com/Vladroon22/GoLog"
	"github.com/go-redis/redis"
)

type Redis struct {
	rd     *redis.Client
	logger *golog.Logger
}

func NewRedis(lg *golog.Logger, cnf *config.Config) *Redis {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:" + cnf.RedisPort,
		Password: "",
		DB:       0,
	})

	if err := client.Ping().Err(); err != nil {
		panic("Failed to ping redis: " + err.Error())
	}

	return &Redis{
		rd:     client,
		logger: lg,
	}
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

func (r *Redis) Iterate() ([]string, error) {
	return r.rd.LRange("jobs", 0, -1).Result()
}
