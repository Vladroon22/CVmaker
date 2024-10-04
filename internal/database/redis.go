package database

import (
	"time"

	golog "github.com/Vladroon22/GoLog"
	"github.com/go-redis/redis"
)

type Redis struct {
	rd     *redis.Client
	logger *golog.Logger
}

func NewRedis(lg *golog.Logger) *Redis {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	return &Redis{
		rd:     client,
		logger: lg,
	}
}

func (r *Redis) SetData(item string, data interface{}, expTime time.Duration) error {
	if err := r.rd.Set(item, data, expTime).Err(); err != redis.Nil {
		r.logger.Errorln(err)
		return err
	}
	return nil
}

func (r *Redis) GetData(item string) string {
	data, err := r.rd.Get(item).Result()
	if err != nil {
		r.logger.Errorln(err)
		return ""
	}
	return data
}

func (r *Redis) Make(a ...interface{}) {
	r.rd.Do(a...).Result()
}

func (r *Redis) Iterate() ([]string, error) {
	return r.rd.LRange("jobs", 0, -1).Result()
}
