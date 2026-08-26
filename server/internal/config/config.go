package config

import (
	"os"
	"time"
)

type Config struct {
	Addr        string
	MySQLDSN    string
	RedisAddr   string
	RedisPass   string
	RedisDB     int
	JWTSecret   string
	AdminUser   string
	AdminPass   string
	TokenTTL    time.Duration
	OriginAllow string
	LogFile     string // 日志落盘路径（空=仅 stdout）
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
		MySQLDSN:    env("KAOSHI_MYSQL_DSN", "root@tcp(127.0.0.1:13306)/kaoshi?charset=utf8mb4&parseTime=True&loc=Local"), // 密码经 DSN 环境变量注入，不入库
		RedisAddr:   env("KAOSHI_REDIS_ADDR", "127.0.0.1:16379"),
		RedisPass:   env("KAOSHI_REDIS_PASS", ""),
		RedisDB:     0,
		JWTSecret:   env("KAOSHI_JWT_SECRET", ""),
		AdminUser:   env("KAOSHI_ADMIN_USER", "admin"),
		AdminPass:   env("KAOSHI_ADMIN_PASS", ""),
		TokenTTL:    24 * time.Hour,
		OriginAllow: env("KAOSHI_ORIGIN_ALLOW", "*"),
		LogFile:     env("KAOSHI_LOG_FILE", ""),
	}
	if cfg.JWTSecret == "" {
		panic("KAOSHI_JWT_SECRET 未设置：拒绝用空 secret 启动（可用 openssl rand -hex 32 生成）")
	}
	if cfg.AdminPass == "" || cfg.AdminPass == "admin123" {
		panic("KAOSHI_ADMIN_PASS 未设置或仍为弱口令：拒绝用默认密码启动（可用 openssl rand -base64 12 生成）")
	}
	return cfg
}
