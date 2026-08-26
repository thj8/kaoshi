package admin

import (
	"strconv"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/auth"
	"kaoshi/internal/middleware"
	"kaoshi/internal/model"
)

// loginLimiter admin 登录失败限速：同 IP 5 次失败锁 1 分钟
var loginLimiter = middleware.NewIPLimiter(5, time.Minute)


type Handler struct {
	DB        *gorm.DB
	AdminUser string
	AdminPass string
}

func New(db *gorm.DB, user, pass string) *Handler {
	return &Handler{DB: db, AdminUser: user, AdminPass: pass}
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": code, "msg": msg, "data": nil})
}

// ---------- 登录 ----------

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if !loginLimiter.Allow(ip) {
		fail(c, 429, "尝试过于频繁，请 1 分钟后再试")
		return
	}
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误")
		return
	}
	if req.Username != h.AdminUser || req.Password != h.AdminPass {
		loginLimiter.Fail(ip)
		fail(c, 401, "用户名或密码错误")
		return
	}
	token, err := auth.Sign(&auth.Claims{Role: auth.RoleAdmin, Nick: "管理员"})
	if err != nil {
		fail(c, 500, "生成令牌失败")
		return
	}
	ok(c, gin.H{"token": token})
}

// ---------- 答题 CRUD ----------

type quizReq struct {
	Title           string `json:"title" binding:"required,max=128"`
	Description     string `json:"description"`
	Mode            string `json:"mode" binding:"required,oneof=normal rush"`
	TotalTime       int    `json:"total_time" binding:"min=0"`
	PerQuestionTime int    `json:"per_question_time" binding:"min=0,max=600"`
	RushEnabled     *bool  `json:"rush_enabled"`
	ShowAnswer      *bool  `json:"show_answer"`
	ShowAnalysis    *bool  `json:"show_analysis"`
	ShowRanking     *bool  `json:"show_ranking"`
	RushWinnerCount int    `json:"rush_winner_count" binding:"min=0,max=10"`
	RushTime        int    `json:"rush_time" binding:"min=0,max=120"`
	RushAnswerTime  int    `json:"rush_answer_time" binding:"min=0,max=600"`
	RushCountdown   *int   `json:"rush_countdown"` // 0=窗口即开（有效值，用指针区分未传；负值引擎钳 0）
	RushBonusScore  int    `json:"rush_bonus_score" binding:"min=0"`
	RushWrongScore  int    `json:"rush_wrong_score" binding:"min=0"`

	ReqScoreSingle     int `json:"req_score_single" binding:"min=0"`
	ReqScoreMultiple   int `json:"req_score_multiple" binding:"min=0"`
	ReqScoreJudge      int `json:"req_score_judge" binding:"min=0"`
	RushScoreSingle    int `json:"rush_score_single" binding:"min=0"`
	RushScoreMultiple  int `json:"rush_score_multiple" binding:"min=0"`
	RushScoreJudge     int `json:"rush_score_judge" binding:"min=0"`
	RushDeductSingle   int `json:"rush_deduct_single" binding:"min=0"`
	RushDeductMultiple int `json:"rush_deduct_multiple" binding:"min=0"`
	RushDeductJudge    int `json:"rush_deduct_judge" binding:"min=0"`
}

func (h *Handler) CreateQuiz(c *gin.Context) {
	var req quizReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误："+err.Error())
		return
	}
	quiz := model.Quiz{
		Title:           req.Title,
		Description:     req.Description,
		Mode:            req.Mode,
		Status:          model.QuizStatusWaiting,
		TotalTime:       req.TotalTime,
		PerQuestionTime: orDefault(req.PerQuestionTime, 30),
		RushEnabled:     boolOr(req.RushEnabled, true), // 抢答与否由题目 required 决定，比赛级默认开启
		ShowAnswer:      boolOr(req.ShowAnswer, true),
		ShowAnalysis:    boolOr(req.ShowAnalysis, true),
		ShowRanking:     boolOr(req.ShowRanking, true),
		RushWinnerCount: orDefault(req.RushWinnerCount, 1),
		RushTime:        orDefault(req.RushTime, 10),
		RushAnswerTime:  orDefault(req.RushAnswerTime, 20),
		RushCountdown:   intOr(req.RushCountdown, 3),
		RushBonusScore:  orDefault(req.RushBonusScore, 5),
		RushWrongScore:  req.RushWrongScore,
		ReqScoreSingle:     req.ReqScoreSingle,
		ReqScoreMultiple:   req.ReqScoreMultiple,
		ReqScoreJudge:      req.ReqScoreJudge,
		RushScoreSingle:    req.RushScoreSingle,
		RushScoreMultiple:  req.RushScoreMultiple,
		RushScoreJudge:     req.RushScoreJudge,
		RushDeductSingle:   req.RushDeductSingle,
		RushDeductMultiple: req.RushDeductMultiple,
		RushDeductJudge:    req.RushDeductJudge,
	}
	if err := h.DB.Create(&quiz).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	ok(c, quiz)
}

func (h *Handler) UpdateQuiz(c *gin.Context) {
	var quiz model.Quiz
	if err := h.DB.First(&quiz, c.Param("id")).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	if quiz.Status != model.QuizStatusWaiting {
		fail(c, 400, "答题已开始，不允许修改配置")
		return
	}
	var req quizReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误："+err.Error())
		return
	}
	quiz.Title = req.Title
	quiz.Description = req.Description
	quiz.Mode = req.Mode
	quiz.TotalTime = req.TotalTime
	quiz.PerQuestionTime = orDefault(req.PerQuestionTime, quiz.PerQuestionTime)
	quiz.RushEnabled = true // 抢答由题目 required 决定，不再提供比赛级开关
	quiz.ShowAnswer = boolOr(req.ShowAnswer, quiz.ShowAnswer)
	quiz.ShowAnalysis = boolOr(req.ShowAnalysis, quiz.ShowAnalysis)
	quiz.ShowRanking = boolOr(req.ShowRanking, quiz.ShowRanking)
	quiz.RushWinnerCount = orDefault(req.RushWinnerCount, quiz.RushWinnerCount)
	quiz.RushTime = orDefault(req.RushTime, quiz.RushTime)
	quiz.RushAnswerTime = orDefault(req.RushAnswerTime, quiz.RushAnswerTime)
	quiz.RushCountdown = intOr(req.RushCountdown, quiz.RushCountdown)
	quiz.RushBonusScore = orDefault(req.RushBonusScore, quiz.RushBonusScore)
	quiz.RushWrongScore = req.RushWrongScore
	quiz.ReqScoreSingle = req.ReqScoreSingle
	quiz.ReqScoreMultiple = req.ReqScoreMultiple
	quiz.ReqScoreJudge = req.ReqScoreJudge
	quiz.RushScoreSingle = req.RushScoreSingle
	quiz.RushScoreMultiple = req.RushScoreMultiple
	quiz.RushScoreJudge = req.RushScoreJudge
	quiz.RushDeductSingle = req.RushDeductSingle
	quiz.RushDeductMultiple = req.RushDeductMultiple
	quiz.RushDeductJudge = req.RushDeductJudge
	h.DB.Save(&quiz)
	ok(c, quiz)
}

func (h *Handler) ListQuizzes(c *gin.Context) {
	var quizzes []model.Quiz
	h.DB.Order("id DESC").Find(&quizzes)
	ok(c, quizzes)
}

func (h *Handler) GetQuiz(c *gin.Context) {
	var quiz model.Quiz
	if err := h.DB.Preload("Questions", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort ASC")
	}).First(&quiz, c.Param("id")).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	ok(c, gin.H{"quiz": quiz, "questions": quiz.Questions})
}

func (h *Handler) DeleteQuiz(c *gin.Context) {
	var quiz model.Quiz
	if err := h.DB.First(&quiz, c.Param("id")).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	if quiz.Status != model.QuizStatusWaiting {
		fail(c, 400, "答题已开始，不允许删除")
		return
	}
	h.DB.Select("Questions").Delete(&quiz) // 级联删题目
	ok(c, nil)
}

// ---------- 题目 CRUD ----------

type optionReq struct {
	Label   string `json:"label"`
	Content string `json:"content" binding:"required"`
}

type questionReq struct {
	Type      string      `json:"type" binding:"required,oneof=single multiple judge"`
	Content   string      `json:"content" binding:"required,max=1024"`
	Options   []optionReq `json:"options" binding:"required,min=2,max=8,dive"`
	Answer    string      `json:"answer" binding:"required"`
	Analysis  string      `json:"analysis"`
	Score     int         `json:"score" binding:"min=0,max=100"`
	Required  *bool       `json:"required"`
	TimeLimit int         `json:"time_limit" binding:"min=0,max=600"`
}

// validateAnswer 校验答案与题型/选项一致性（多选自动按字母排序）
func normalizeAnswer(req *questionReq) (string, bool) {
	labels := map[string]bool{}
	for _, o := range req.Options {
		labels[o.Label] = true
	}
	switch req.Type {
	case model.QuestionTypeSingle, model.QuestionTypeJudge:
		if len(req.Answer) == 1 && labels[req.Answer] {
			return req.Answer, true
		}
	case model.QuestionTypeMultiple:
		ans := []rune(req.Answer)
		// 排序去重
		for i := 0; i < len(ans); i++ {
			for j := i + 1; j < len(ans); j++ {
				if ans[i] > ans[j] {
					ans[i], ans[j] = ans[j], ans[i]
				}
			}
		}
		dedup := string(ans)
		if dedup == "" {
			return "", false
		}
		for i := 0; i < len(dedup); i++ {
			seen := false
			for _, o := range req.Options {
				if string(dedup[i]) == o.Label {
					seen = true
					break
				}
			}
			if !seen {
				return "", false
			}
		}
		return dedup, true
	}
	return "", false
}

func buildOptions(q *model.Question, req *questionReq) []model.QuestionOption {
	opts := make([]model.QuestionOption, 0, len(req.Options))
	for i, o := range req.Options {
		label := o.Label
		if label == "" {
			label = string(rune('A' + i))
		}
		opts = append(opts, model.QuestionOption{
			QuestionID: q.ID,
			Label:      label,
			Content:    o.Content,
			Sort:       i,
		})
	}
	return opts
}

func (h *Handler) CreateQuestion(c *gin.Context) {
	quizID := c.Param("id")
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		fail(c, 404, "答题不存在")
		return
	}
	if quiz.Status != model.QuizStatusWaiting {
		fail(c, 400, "答题已开始，不允许添加题目")
		return
	}
	var req questionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误："+err.Error())
		return
	}
	answer, valid := normalizeAnswer(&req)
	if !valid {
		fail(c, 400, "正确答案与选项不匹配")
		return
	}

	var maxSort int
	h.DB.Model(&model.Question{}).Where("quiz_id = ?", quiz.ID).Select("COALESCE(MAX(sort),0)").Scan(&maxSort)

	q := model.Question{
		QuizID:    quiz.ID,
		Type:      req.Type,
		Content:   req.Content,
		Answer:    answer,
		Analysis:  req.Analysis,
		Score:     orDefault(req.Score, 10),
		Required:  boolOr(req.Required, true),
		Sort:      maxSort + 1,
		TimeLimit: req.TimeLimit,
	}
	if err := h.DB.Create(&q).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	h.DB.Create(buildOptions(&q, &req))
	ok(c, q)
}

func (h *Handler) ListQuestions(c *gin.Context) {
	quizID := c.Param("id")
	var questions []model.Question
	h.DB.Where("quiz_id = ?", quizID).Order("sort ASC").Find(&questions)
	if len(questions) == 0 {
		ok(c, []any{})
		return
	}
	ids := make([]int64, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}
	var allOptions []model.QuestionOption
	h.DB.Where("question_id IN ?", ids).Order("sort ASC").Find(&allOptions)
	optsMap := map[int64][]model.QuestionOption{}
	for _, o := range allOptions {
		optsMap[o.QuestionID] = append(optsMap[o.QuestionID], o)
	}
	type qWithOpts struct {
		model.Question
		Options []model.QuestionOption `json:"options"`
		// 管理端需要编辑答案与解析，单独暴露
		AnswerStr string `json:"answer"`
	}
	out := make([]qWithOpts, len(questions))
	for i := range questions {
		out[i] = qWithOpts{Question: questions[i], Options: optsMap[questions[i].ID], AnswerStr: questions[i].Answer}
	}
	ok(c, out)
}

func (h *Handler) UpdateQuestion(c *gin.Context) {
	var q model.Question
	if err := h.DB.First(&q, c.Param("qid")).Error; err != nil {
		fail(c, 404, "题目不存在")
		return
	}
	var quiz model.Quiz
	if err := h.DB.First(&quiz, q.QuizID).Error; err != nil || quiz.Status != model.QuizStatusWaiting {
		fail(c, 400, "答题已开始，不允许修改题目")
		return
	}
	var req questionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数错误："+err.Error())
		return
	}
	answer, valid := normalizeAnswer(&req)
	if !valid {
		fail(c, 400, "正确答案与选项不匹配")
		return
	}
	q.Type = req.Type
	q.Content = req.Content
	q.Answer = answer
	q.Analysis = req.Analysis
	q.Score = orDefault(req.Score, q.Score)
	q.Required = boolOr(req.Required, q.Required)
	q.TimeLimit = req.TimeLimit
	h.DB.Save(&q)
	h.DB.Where("question_id = ?", q.ID).Delete(&model.QuestionOption{})
	h.DB.Create(buildOptions(&q, &req))
	ok(c, q)
}

func (h *Handler) DeleteQuestion(c *gin.Context) {
	var q model.Question
	if err := h.DB.First(&q, c.Param("qid")).Error; err != nil {
		fail(c, 404, "题目不存在")
		return
	}
	var quiz model.Quiz
	if err := h.DB.First(&quiz, q.QuizID).Error; err != nil || quiz.Status != model.QuizStatusWaiting {
		fail(c, 400, "答题已开始，不允许删除题目")
		return
	}
	h.DB.Where("question_id = ?", q.ID).Delete(&model.QuestionOption{})
	h.DB.Delete(&q)
	ok(c, nil)
}

// ---------- 工具 ----------

func orDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func intOr(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// ---------- 参赛用户（邀请名单） ----------

// ListInvitees GET /api/admin/quiz/:id/invitees
func (h *Handler) ListInvitees(c *gin.Context) {
	quizID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var rows []model.QuizInvitee
	h.DB.Where("quiz_id = ?", quizID).Order("user_id").Find(&rows)
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
	}
	var users []model.User
	if len(ids) > 0 {
		h.DB.Where("id IN ?", ids).Find(&users)
	}
	um := map[int64]model.User{}
	for _, u := range users {
		um[u.ID] = u
	}
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		if u, ok := um[r.UserID]; ok {
			items = append(items, gin.H{"user_id": r.UserID, "username": u.Username, "nickname": u.Nickname})
		}
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"items": items}})
}

type inviteesReq struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

// SetInvitees PUT /api/admin/quiz/:id/invitees 全量替换（仅 WAITING）
func (h *Handler) SetInvitees(c *gin.Context) {
	quizID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		c.JSON(200, gin.H{"code": 404, "msg": "答题不存在"})
		return
	}
	if quiz.Status != model.QuizStatusWaiting {
		c.JSON(200, gin.H{"code": 400, "msg": "比赛已开始，不能修改参赛名单"})
		return
	}
	var req inviteesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"code": 400, "msg": "缺少 user_ids"})
		return
	}
	// 去重
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(req.UserIDs))
	for _, id := range req.UserIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	// 校验全部存在（任一不存在整单拒绝）
	var n int64
	h.DB.Model(&model.User{}).Where("id IN ?", ids).Count(&n)
	if n != int64(len(ids)) {
		c.JSON(200, gin.H{"code": 400, "msg": "存在无效用户"})
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("quiz_id = ?", quizID).Delete(&model.QuizInvitee{}).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		now := time.Now()
		rows := make([]model.QuizInvitee, len(ids))
		for i, id := range ids {
			rows[i] = model.QuizInvitee{QuizID: quizID, UserID: id, CreatedAt: now}
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		c.JSON(200, gin.H{"code": 500, "msg": "保存失败"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "ok"})
}
