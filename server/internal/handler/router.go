package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"kaoshi/internal/config"
	"kaoshi/internal/engine"
	"kaoshi/internal/handler/admin"
	"kaoshi/internal/handler/api"
	"kaoshi/internal/middleware"
	"kaoshi/internal/ws"
)

// Register 组装全部路由
func Register(r *gin.Engine, cfg *config.Config, db *gorm.DB, rdb *redis.Client) *engine.Engine {
	adminH := admin.New(db, cfg.AdminUser, cfg.AdminPass)

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

	// WebSocket
	hub := ws.NewHub()
	eng := engine.New(db, rdb, hub)
	wsSrv := &ws.Server{Hub: hub}
	wsSrv.Snapshot = eng.Snapshot
	r.GET("/ws", wsSrv.HandleWS)

	// 公开接口（注册/登录/brief）
	answerH := api.NewAnswer(db, eng)
	apiH := api.New(db, cfg.JWTSecret)
	r.POST("/api/auth/login", apiH.Login)
	r.GET("/api/quiz/:id/brief", apiH.QuizBrief)

	// 用户端（需登录）
	user := r.Group("/api", middleware.UserAuth())
	{
		user.GET("/auth/me", apiH.Me)
		user.GET("/quizzes", apiH.QuizList)
		user.POST("/join", apiH.Join)
		user.GET("/quiz/:id", apiH.QuizInfo)
		user.GET("/quiz/:id/current-question", answerH.CurrentQuestion)
		user.POST("/question/:id/answer", answerH.Submit)
		user.POST("/question/:id/rush", answerH.Rush)
		user.GET("/quiz/:id/ranking", answerH.Ranking)
		user.GET("/quiz/:id/result", answerH.Result)
	}

	// 管理端
	adm := r.Group("/api/admin")
	adm.POST("/login", adminH.Login)

	authed := adm.Group("", middleware.AdminAuth())
	{
		authed.GET("/users", adminH.ListUsers)
		authed.POST("/users", adminH.CreateUser)
		authed.GET("/users/:id", adminH.UserDetail)
		authed.PUT("/users/:id", adminH.UpdateUser)
		authed.DELETE("/users/:id", adminH.DeleteUser)

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

	// 管理流程控制
	ctrl := admin.NewControl(db, eng)
	ctrlGroup := adm.Group("", middleware.AdminAuth())
	{
		ctrlGroup.POST("/quiz/:id/start", ctrl.Start)
		ctrlGroup.POST("/quiz/:id/pause", ctrl.Pause)
		ctrlGroup.POST("/quiz/:id/resume", ctrl.Resume)
		ctrlGroup.POST("/quiz/:id/next", ctrl.Next)
		ctrlGroup.POST("/quiz/:id/previous", ctrl.Previous)
		ctrlGroup.POST("/quiz/:id/reveal", ctrl.Reveal)
		ctrlGroup.POST("/quiz/:id/end", ctrl.End)
		ctrlGroup.POST("/quiz/:id/rush/start", ctrl.RushStart)
		ctrlGroup.POST("/quiz/:id/rush/end", ctrl.RushEnd)
		ctrlGroup.GET("/quiz/:id/statistics", ctrl.Statistics)
	}
	return eng
}
