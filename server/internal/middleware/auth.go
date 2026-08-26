package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"kaoshi/internal/auth"

	"gorm.io/gorm"
)

// BearerToken 从 Authorization 头提取 token
func BearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// AdminAuth 管理端鉴权中间件
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := auth.Parse(BearerToken(c))
		if err != nil || claims.Role != auth.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录或登录已过期"})
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}

// UserAuth 用户端鉴权中间件
func UserAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := auth.Parse(BearerToken(c))
		if err != nil || claims.Role != auth.RoleUser {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "请先加入答题"})
			return
		}
		// token 内 user 必须仍存在（防清库/重置后旧 token 静默产生孤儿数据）
		var n int64
		db.Table("users").Where("id = ?", claims.UserID).Count(&n)
		if n == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "账号已失效，请重新登录"})
			return
		}
		c.Set("claims", claims)
		c.Set("uid", claims.UserID)
		c.Next()
	}
}
