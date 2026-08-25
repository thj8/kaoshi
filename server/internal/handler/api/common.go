package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler 用户端公共依赖
type Handler struct {
	DB     *gorm.DB
	Secret string
}

func New(db *gorm.DB, secret string) *Handler {
	return &Handler{DB: db, Secret: secret}
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg, "data": nil})
}
