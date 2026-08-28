package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/auth"
	"kaoshi/internal/model"
)

// quizByCode 对外 code 寻址（自增 id 不再对外服务任何接口）
func quizByCode(db *gorm.DB, code string) (*model.Quiz, bool) {
	var q model.Quiz
	if err := db.Where("code = ?", code).First(&q).Error; err != nil {
		return nil, false
	}
	return &q, true
}

func quizBriefOf(q *model.Quiz) gin.H {
	return gin.H{
		"id": q.ID, "code": q.Code, "title": q.Title, "description": q.Description,
		"status": q.Status, "mode": q.Mode,
		"show_answer": q.ShowAnswer, "show_analysis": q.ShowAnalysis, "show_ranking": q.ShowRanking,
	}
}

// QuizBrief GET /api/quiz/:id/brief 公开信息（加入页展示，无需登录；不含任何答案信息）
func (h *Handler) QuizBrief(c *gin.Context) {
	var quiz model.Quiz
	if err := h.DB.Where("code = ?", c.Param("id")).First(&quiz).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	var count int64
	h.DB.Model(&model.Participant{}).Where("quiz_id = ?", quiz.ID).Count(&count)
	ok(c, gin.H{
		"id":                quiz.ID,
		"code":              quiz.Code,
		"title":             quiz.Title,
		"description":       quiz.Description,
		"status":            quiz.Status,
		"mode":              quiz.Mode,
		"participant_count": count,
	})
}

// QuizList GET /api/quizzes 可加入的活动列表（仅 WAITING；受限且未受邀的不可见）
func (h *Handler) QuizList(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var quizzes []model.Quiz
	h.DB.Where("status = ?", model.QuizStatusWaiting).Order("id DESC").Limit(100).Find(&quizzes)
	items := make([]gin.H, 0, len(quizzes))
	for _, q := range quizzes {
		// 受限且未受邀 → 不可见
		var invAll, invMe int64
		h.DB.Model(&model.QuizInvitee{}).Where("quiz_id = ?", q.ID).Count(&invAll)
		if invAll > 0 {
			h.DB.Model(&model.QuizInvitee{}).Where("quiz_id = ? AND user_id = ?", q.ID, claims.UserID).Count(&invMe)
			if invMe == 0 {
				continue
			}
		}
		var cnt, joined int64
		h.DB.Model(&model.Participant{}).Where("quiz_id = ?", q.ID).Count(&cnt)
		h.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", q.ID, claims.UserID).Count(&joined)
		items = append(items, gin.H{
		"id": q.ID, "code": q.Code, "title": q.Title, "description": q.Description,
			"mode": q.Mode, "participant_count": cnt,
			"joined": joined > 0,
		})
	}
	ok(c, gin.H{"items": items})
}

// MyQuizzes GET /api/my/quizzes 我参加过的全部比赛（含已结束）
func (h *Handler) MyQuizzes(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	rows := []struct {
		model.Participant
		Title  string
		Status string
		Mode   string
		Code   string
	}{}
	h.DB.Table("participants").
		Select("participants.*, quizzes.title AS title, quizzes.status AS status, quizzes.mode AS mode, quizzes.code AS code").
		Joins("JOIN quizzes ON quizzes.id = participants.quiz_id").
		Where("participants.user_id = ?", claims.UserID).
		Order("quizzes.status = 'FINISHED', quizzes.id DESC").
		Limit(200).Scan(&rows)
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		var cnt int64
		h.DB.Model(&model.Participant{}).Where("quiz_id = ?", r.QuizID).Count(&cnt)
		items = append(items, gin.H{
			"quiz_id": r.QuizID, "code": r.Code, "title": r.Title, "status": r.Status, "mode": r.Mode,
			"score": r.Score, "correct": r.CorrectCount, "wrong": r.WrongCount,
			"joined_at": r.JoinedAt, "participant_count": cnt,
		})
	}
	ok(c, gin.H{"items": items})
}

// QuizInfo GET /api/quiz/:id 答题基础信息（需用户 token）
func (h *Handler) QuizInfo(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var quiz model.Quiz
	if err := h.DB.Where("code = ?", c.Param("id")).First(&quiz).Error; err != nil {
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
