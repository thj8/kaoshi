package api

import (
	"github.com/gin-gonic/gin"

	"kaoshi/internal/auth"
	"kaoshi/internal/model"
)

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
		"id":                quiz.ID,
		"title":             quiz.Title,
		"description":       quiz.Description,
		"status":            quiz.Status,
		"participant_count": count,
	})
}

// QuizList GET /api/quizzes 可加入的活动列表（仅 WAITING，供选手端展示点击）
func (h *Handler) QuizList(c *gin.Context) {
	var quizzes []model.Quiz
	h.DB.Where("status = ?", model.QuizStatusWaiting).Order("id DESC").Limit(100).Find(&quizzes)
	items := make([]gin.H, 0, len(quizzes))
	for _, q := range quizzes {
		var cnt int64
		h.DB.Model(&model.Participant{}).Where("quiz_id = ?", q.ID).Count(&cnt)
		items = append(items, gin.H{
			"id": q.ID, "title": q.Title, "description": q.Description,
			"mode": q.Mode, "participant_count": cnt,
		})
	}
	ok(c, gin.H{"items": items})
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
