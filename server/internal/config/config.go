package config

import (
	"os"
	"time"
)

type Config struct {
	Addr        string
	MySQLDSN    string
	RedisAddr   string
	RedisDB     int
	JWTSecret   string
	AdminUser   string
	AdminPass   string
	TokenTTL    time.Duration
	OriginAllow string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() *Config {
	cfg := &Config{
		Addr:        env("KAOSHI_ADDR", ":8080"),
		MySQLDSN:    env("KAOSHI_MYSQL_DSN", "root:root123456@tcp(127.0.0.1:13306)/kaoshi?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:   env("KAOSHI_REDIS_ADDR", "127.0.0.1:16379"),
		RedisDB:     0,
		JWTSecret:   env("KAOSHI_JWT_SECRET", ""),
		AdminUser:   env("KAOSHI_ADMIN_USER", "admin"),
		AdminPass:   env("KAOSHI_ADMIN_PASS", "admin123"),
		TokenTTL:    24 * time.Hour,
		OriginAllow: env("KAOSHI_ORIGIN_ALLOW", "*"),
	}
	if cfg.JWTSecret == "" {
		panic("KAOSHI_JWT_SECRET 未设置：拒绝用空 secret 启动（可用 openssl rand -hex 32 生成）")
	}
	return cfg
}
