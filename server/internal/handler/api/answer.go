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
	quiz := c.MustGet("quiz").(*model.Quiz)
	if quiz.Mode == model.ModeExam {
		// 考试（自由切题）模式排行榜仅管理员可见（控制台/大屏走 /api/admin/quiz/:id/statistics），
		// 选手不可互看实时得分与排名，防互抄与踩点
		fail(c, 403, "考试模式排行榜仅管理员可见")
		return
	}
	items := h.Eng.Ranking(quiz.ID, 100)
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
	quiz := c.MustGet("quiz").(*model.Quiz)
	quizID := quiz.ID

	var p model.Participant
	if err := h.DB.Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).First(&p).Error; err != nil {
		fail(c, 404, "未参加该答题")
		return
	}
	// 考试模式未交卷：不出实时对错/得分计数 —— 否则脚本可「逐题保存选项→拉 result 看对错数是否变化」
	// 逐题试探猜答案，击穿「考试逐题不回传对错」的防试答设计（E11 只封了逐题接口，result 必须同封）。
	// 交卷后成绩锁定（FinalizePaper），对错计数不再变化，可安全返回。
	if quiz.Mode == model.ModeExam && p.FinishedAt == nil {
		fail(c, 403, "交卷后才能查看成绩")
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
	// 考试模式：用时 = 首次保存答案 → 交卷（未交卷则至今）的墙钟时间。
	// 不能逐题 duration 累加：自由切题下各题计时窗口 [首次打开, 最近保存] 相互重叠
	// （来回切题/改答），累加会显著虚高（如 30 分钟考试算出 50+ 分钟）。
	if quiz.Mode == model.ModeExam {
		durationMs = examWallDurationMs(h.DB, quizID, claims.UserID, p.FinishedAt)
	}

	ok(c, gin.H{
		"nickname": claims.Nick,
		"score":    p.Score,
		"correct":  correct,
		"wrong":    answered - correct,
		"answered": answered,
		"total":    totalQ,
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
	quiz := c.MustGet("quiz").(*model.Quiz)
	q := h.Eng.CurrentQuestionInfo(quiz.ID)
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
