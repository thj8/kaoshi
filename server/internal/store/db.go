package store

import (
	"context"
	"log"
	"time"

	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"kaoshi/internal/model"
)

// NewDB 初始化 MySQL 连接（含重试，容器启动时 mysql 可能未就绪）
func NewDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(gmysql.Open(dsn), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		})
		if err == nil {
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(50)
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetConnMaxLifetime(time.Hour)
			if err = sqlDB.PingContext(context.Background()); err == nil {
				break
			}
		}
		log.Printf("waiting mysql... (%d/30) %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return nil, err
	}

	// 旧数据回填对外随机码
	var quizzes []model.Quiz
	db.Where("code IS NULL OR code = ''").Find(&quizzes)
	for _, q := range quizzes {
		db.Model(&model.Quiz{}).Where("id = ?", q.ID).Update("code", model.NewQuizCode())
	}
	return db, nil
}
