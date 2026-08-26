package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"kaoshi/internal/auth"
	"kaoshi/internal/config"
	router "kaoshi/internal/handler"
	"kaoshi/internal/middleware"
	"kaoshi/internal/store"
)

func main() {
	cfg := config.Load()
	auth.Init(cfg.JWTSecret, cfg.TokenTTL)

	// 日志同时落盘（stdout 保留，docker logs 仍可用）
	if cfg.LogFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0o755); err != nil {
			log.Fatalf("init log dir failed: %v", err)
		}
		// ponytail: 追加单文件无轮转，量大了再上 lumberjack/logrotate
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("open log file failed: %v", err)
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	db, err := store.NewDB(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	rdb, err := store.NewRedis(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
	if err != nil {
		log.Fatalf("init redis failed: %v", err)
	}
	_ = rdb

	if os.Getenv("KAOSHI_ENV") == "dev" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS(cfg.OriginAllow))

	router.Register(r, cfg, db, rdb)

	srv := &http.Server{Addr: cfg.Addr, Handler: r}
	go func() {
		log.Printf("server listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
