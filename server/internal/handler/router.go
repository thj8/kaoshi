package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/config"
	"kaoshi/internal/handler/admin"
	"kaoshi/internal/middleware"
)

// Register 组装全部路由
func Register(r *gin.Engine, cfg *config.Config, db *gorm.DB) {
	adminH := admin.New(db, cfg.AdminUser, cfg.AdminPass)

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

	api := r.Group("/api")

	// 管理端
	adm := api.Group("/admin")
	adm.POST("/login", adminH.Login)

	authed := adm.Group("", middleware.AdminAuth())
	{
		authed.GET("/quizzes", adminH.ListQuizzes)
		authed.POST("/quiz", adminH.CreateQuiz)
		authed.GET("/quiz/:id", adminH.GetQuiz)
		authed.PUT("/quiz/:id", adminH.UpdateQuiz)
		authed.DELETE("/quiz/:id", adminH.DeleteQuiz)
		authed.POST("/quiz/:id/questions", adminH.CreateQuestion)
		authed.GET("/quiz/:id/questions", adminH.ListQuestions)
		authed.PUT("/question/:qid", adminH.UpdateQuestion)
		authed.DELETE("/question/:qid", adminH.DeleteQuestion)
	}

	// 用户端路由（阶段 3 注册）
}
