package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"kaoshi/internal/auth"
	"kaoshi/internal/model"

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

// QuizScope 用户端 quiz 作用域校验：路径参数 :id 为比赛码，token 的 quiz_id 必须匹配。
// 统一收拢原先散在 6 个 handler 里的 quizByCode+越权卫兵；通过后 c.Set("quiz", *model.Quiz)。
func QuizScope(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := c.MustGet("claims").(*auth.Claims)
		var quiz model.Quiz
		if err := db.Where("code = ?", c.Param("id")).First(&quiz).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 404, "msg": "答题不存在"})
			return
		}
		if claims.QuizID != quiz.ID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "只能访问自己参加的答题"})
			return
		}
		c.Set("quiz", &quiz)
		c.Next()
	}
}
