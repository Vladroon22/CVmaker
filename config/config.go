package config

import "os"

type Config struct {
	Addr_PORT string
	DB        string
	JWT       string
}

func CreateConfig() *Config {
	return &Config{
		Addr_PORT: getEnv("addr_port", ""),
		DB:        getEnv("DB", ""),
		JWT:       getEnv("KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
