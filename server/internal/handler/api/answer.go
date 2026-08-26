package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/auth"
	"kaoshi/internal/engine"
	"kaoshi/internal/model"
)

type AnswerHandler struct {
	DB  *gorm.DB
	Eng *engine.Engine
}

func NewAnswer(db *gorm.DB, eng *engine.Engine) *AnswerHandler {
	return &AnswerHandler{DB: db, Eng: eng}
}

type answerReq struct {
	Answer   string `json:"answer" binding:"required"`
	Duration int    `json:"duration"` // 用时毫秒（客户端参考值）
}

// Submit POST /api/question/:id/answer
func (h *AnswerHandler) Submit(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	questionID, _ := parseInt64(c.Param("id"))
	var req answerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "请选择答案")
		return
	}
	result, err := h.Eng.SubmitAnswer(claims.QuizID, questionID, claims.UserID, req.Answer, req.Duration)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, result)
}

// Ranking GET /api/quiz/:id/ranking
func (h *AnswerHandler) Ranking(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quizID, _ := parseInt64(c.Param("id"))
	if quizID != claims.QuizID {
		fail(c, 403, "只能查看自己参加的答题")
		return
	}
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	items := h.Eng.Ranking(quizID, 100)
	if !quiz.ShowRanking {
		// 关闭排行时不暴露榜单，仅返回自己排名
		mine := gin.H{"visible": false}
		for _, it := range items {
			if it.UserID == claims.UserID {
				mine["mine"] = it
				break
			}
		}
		ok(c, mine)
		return
	}
	mineRank := 0
	for _, it := range items {
		if it.UserID == claims.UserID {
			mineRank = it.Rank
			break
		}
	}
	ok(c, gin.H{"visible": true, "items": items, "mine_rank": mineRank})
}

// Result GET /api/quiz/:id/result 个人成绩（结束后）
func (h *AnswerHandler) Result(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quizID, _ := parseInt64(c.Param("id"))
	if quizID != claims.QuizID {
		fail(c, 403, "只能查看自己参加的答题")
		return
	}
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}

	var p model.Participant
	if err := h.DB.Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).First(&p).Error; err != nil {
		fail(c, 404, "未参加该答题")
		return
	}

	var answered, correct int64
	h.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).Count(&answered)
	h.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ? AND is_correct = ?", quizID, claims.UserID, true).Count(&correct)

	var totalQ int64
	h.DB.Model(&model.Question{}).Where("quiz_id = ?", quizID).Count(&totalQ)

	// 排名
	rank := h.Eng.UserRank(quizID, claims.UserID)

	var durationMs int64
	h.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).
		Select("COALESCE(SUM(duration),0)").Scan(&durationMs)

	ok(c, gin.H{
		"nickname":     claims.Nick,
		"score":        p.Score,
		"correct":      correct,
		"wrong":        answered - correct,
		"answered":     answered,
		"total":        totalQ,
		"correct_rate": func() float64 {
			if answered == 0 {
				return 0
			}
			return float64(correct) / float64(answered) * 100
		}(),
		"duration_sec": durationMs / 1000,
		"rank":         rank,
		"finished":     quiz.Status == model.QuizStatusFinished,
		"ended_at":     quiz.EndedAt,
		"started_at":   quiz.StartedAt,
		"server_now":   time.Now().Unix(),
	})
}

// Rush POST /api/question/:id/rush 抢答（服务器原子判序）
func (h *AnswerHandler) Rush(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	questionID, _ := parseInt64(c.Param("id"))
	// 发起日志：接口调用即记（结果日志由引擎补，两条一对）
	log.Printf("[rush] quiz=%d q=%d %s 发起抢答", claims.QuizID, questionID, claims.Nick)
	result, err := h.Eng.RushSubmit(claims.QuizID, questionID, claims.UserID)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, result)
}

// CurrentQuestion GET /api/quiz/:id/current-question 当前题（REST 兑底，刷新恢复）
func (h *AnswerHandler) CurrentQuestion(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quizID, _ := parseInt64(c.Param("id"))
	if quizID != claims.QuizID {
		fail(c, 403, "只能查看自己参加的答题")
		return
	}
	q := h.Eng.CurrentQuestionInfo(quizID)
	if q == nil {
		ok(c, nil)
		return
	}
	ok(c, q)
}

func parseInt64(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	n := int64(0)
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int64(ch-'0')
	}
	return n, true
}
