package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/auth"
	"kaoshi/internal/model"
)

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

type joinReq struct {
	Nickname string `json:"nickname" binding:"required,min=1,max=32"`
	QuizID   int64  `json:"quiz_id" binding:"required"`
}

// Join 输入昵称加入指定答题，返回 token
func (h *Handler) Join(c *gin.Context) {
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "请输入昵称与答题编号")
		return
	}
	var quiz model.Quiz
	if err := h.DB.First(&quiz, req.QuizID).Error; err != nil {
		fail(c, 404, "答题不存在，请检查链接")
		return
	}
	if quiz.Status == model.QuizStatusFinished {
		fail(c, 400, "答题已结束")
		return
	}

	// 创建/复用用户（同昵称视为同一用户）
	var user model.User
	if err := h.DB.Where("nickname = ?", req.Nickname).First(&user).Error; err != nil {
		user = model.User{Nickname: req.Nickname}
		if err := h.DB.Create(&user).Error; err != nil {
			fail(c, 500, "创建用户失败")
			return
		}
	}

	// 幂等加入
	var p model.Participant
	if err := h.DB.Where("quiz_id = ? AND user_id = ?", quiz.ID, user.ID).First(&p).Error; err != nil {
		p = model.Participant{QuizID: quiz.ID, UserID: user.ID, JoinedAt: time.Now()}
		h.DB.Create(&p)
	}

	token, err := auth.Sign(&auth.Claims{
		Role:   auth.RoleUser,
		UserID: user.ID,
		Nick:   user.Nickname,
		QuizID: quiz.ID,
	})
	if err != nil {
		fail(c, 500, "生成令牌失败")
		return
	}
	ok(c, gin.H{
		"token":  token,
		"quiz":   quizBriefOf(&quiz),
		"user":   gin.H{"id": user.ID, "nickname": user.Nickname},
	})
}

func quizBriefOf(q *model.Quiz) gin.H {
	return gin.H{
		"id": q.ID, "title": q.Title, "description": q.Description,
		"status": q.Status, "mode": q.Mode,
		"show_answer": q.ShowAnswer, "show_analysis": q.ShowAnalysis, "show_ranking": q.ShowRanking,
	}
}

// QuizBrief GET /api/quiz/:id/brief 公开信息（加入页展示，无需登录；不含任何答案信息）
func (h *Handler) QuizBrief(c *gin.Context) {
	var quiz model.Quiz
	if err := h.DB.First(&quiz, c.Param("id")).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	var count int64
	h.DB.Model(&model.Participant{}).Where("quiz_id = ?", quiz.ID).Count(&count)
	ok(c, gin.H{
		"id":              quiz.ID,
		"title":           quiz.Title,
		"description":     quiz.Description,
		"status":          quiz.Status,
		"participant_count": count,
	})
}

// QuizInfo GET /api/quiz/:id 答题基础信息（需用户 token）
func (h *Handler) QuizInfo(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var quiz model.Quiz
	if err := h.DB.First(&quiz, c.Param("id")).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	var count int64
	h.DB.Model(&model.Participant{}).Where("quiz_id = ?", quiz.ID).Count(&count)
	ok(c, gin.H{
		"quiz":              quizBriefOf(&quiz),
		"participant_count": count,
		"me": gin.H{
			"user_id":  claims.UserID,
			"nickname": claims.Nick,
		},
	})
}
