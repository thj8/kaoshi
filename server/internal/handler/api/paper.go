package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/auth"
	"kaoshi/internal/engine"
	"kaoshi/internal/model"
)

// PaperHandler 考试（自由切题）模式试卷接口
type PaperHandler struct {
	DB  *gorm.DB
	Eng *engine.Engine
}

func NewPaper(db *gorm.DB, eng *engine.Engine) *PaperHandler {
	return &PaperHandler{DB: db, Eng: eng}
}

// Paper GET /api/quiz/:id/paper 全卷下发（答案/解析绝不入参；含本人已存答案）
func (h *PaperHandler) Paper(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quiz := c.MustGet("quiz").(*model.Quiz)
	if quiz.Mode != model.ModeExam {
		fail(c, 400, "非考试模式")
		return
	}

	// 未开始不下发题目内容（防提前看题）；已结束后保留供回看
	var qs []model.Question
	if quiz.Status != model.QuizStatusWaiting {
		h.DB.Where("quiz_id = ?", quiz.ID).Order("sort ASC").Find(&qs)
	}
	// 题目总数（仅数量不含内容，等待页展示用；total 为本轮实际下发的题数）
	var questionCount int64
	h.DB.Model(&model.Question{}).Where("quiz_id = ?", quiz.ID).Count(&questionCount)

	// 选项与本人已存答案（一次批量查询）；未答为 null
	options := map[int64][]gin.H{}
	myAns := map[int64]*string{}
	if len(qs) > 0 {
		ids := make([]int64, 0, len(qs))
		for _, q := range qs {
			ids = append(ids, q.ID)
		}
		var opts []model.QuestionOption
		h.DB.Where("question_id IN ?", ids).Order("sort ASC").Find(&opts)
		for _, o := range opts {
			options[o.QuestionID] = append(options[o.QuestionID], gin.H{"label": o.Label, "content": o.Content})
		}
		var answers []model.Answer
		h.DB.Where("quiz_id = ? AND user_id = ?", quiz.ID, claims.UserID).Find(&answers)
		for _, a := range answers {
			v := a.Answer
			myAns[a.QuestionID] = &v
		}
	}

	items := make([]gin.H, 0, len(qs))
	for i, q := range qs {
		items = append(items, gin.H{
			"id":        q.ID,
			"index":     i + 1,
			"type":      q.Type,
			"content":   q.Content,
			"score":     q.Score,
			"options":   options[q.ID],
			"my_answer": myAns[q.ID],
		})
	}

	var p model.Participant
	submitted := false
	if err := h.DB.Where("quiz_id = ? AND user_id = ?", quiz.ID, claims.UserID).First(&p).Error; err == nil {
		submitted = p.FinishedAt != nil
	}

	// 考试截止 = 开始时间 + 总时长（服务端唯一事实来源）
	var deadline int64
	if quiz.Status == model.QuizStatusRunning && quiz.StartedAt != nil && quiz.TotalTime > 0 {
		deadline = quiz.StartedAt.Add(time.Duration(quiz.TotalTime) * time.Second).UnixMilli()
	}

	ok(c, gin.H{
		"title":          quiz.Title,
		"mode":           quiz.Mode,
		"status":         quiz.Status,
		"total":          len(qs),
		"question_count": questionCount,
		"deadline_at":    deadline,
		"server_now":     time.Now().UnixMilli(),
		"submitted":      submitted,
		"score":          p.Score,
		"questions":      items,
	})
}

type paperAnswerReq struct {
	QuestionID int64  `json:"question_id" binding:"required"`
	Answer     string `json:"answer"` // 允许空串 = 清除草稿（多选逐一取消后全空）
	Duration   int    `json:"duration"`
}

// SavePaperAnswer POST /api/quiz/:id/paper/answer 选择即保存（可修改，直到交卷/到时）
func (h *PaperHandler) SavePaperAnswer(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quiz := c.MustGet("quiz").(*model.Quiz)
	var req paperAnswerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误")
		return
	}
	result, err := h.Eng.SavePaperAnswer(quiz.ID, claims.UserID, req.QuestionID, req.Answer, req.Duration)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, result)
}

// SubmitPaper POST /api/quiz/:id/paper/submit 交卷（幂等）
func (h *PaperHandler) SubmitPaper(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	quiz := c.MustGet("quiz").(*model.Quiz)
	result, err := h.Eng.FinalizePaper(quiz.ID, claims.UserID)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, result)
}

// examWallDurationMs 考试模式答题用时：本人首次保存答案到交卷（未交卷则到当前）的墙钟毫秒数。
// 不能用逐题 duration 累加：自由切题下各题计时窗口相互重叠（来回切题/改答），累加会虚高。
func examWallDurationMs(db *gorm.DB, quizID, userID int64, finishedAt *time.Time) int64 {
	var first sql.NullTime
	row := db.Raw("SELECT MIN(submitted_at) FROM answers WHERE quiz_id = ? AND user_id = ?", quizID, userID).Row()
	_ = row.Scan(&first)
	if !first.Valid {
		return 0 // 一题都没答过
	}
	end := time.Now()
	if finishedAt != nil {
		end = *finishedAt
	}
	if end.Before(first.Time) {
		return 0
	}
	return int64(end.Sub(first.Time) / time.Millisecond)
}
