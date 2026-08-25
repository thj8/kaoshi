package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	db, err := store.NewDB(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("init mysql failed: %v", err)
	}
	rdb, err := store.NewRedis(cfg.RedisAddr, cfg.RedisDB)
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

	router.Register(r, cfg, db)

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
