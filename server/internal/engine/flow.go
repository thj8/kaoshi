package engine

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kaoshi/internal/model"
	"kaoshi/internal/ws"
)

var (
	ErrNotWaiting   = errors.New("答题已开始")
	ErrNotRunning   = errors.New("答题未在进行中")
	ErrNotFound     = errors.New("答题或题目不存在")
	ErrAlreadyEnded = errors.New("答题已结束")
)

// AnswerUnanswered 未答标记（必答题超时记录）
const AnswerUnanswered = "-"

// ---------- 状态流转 ----------

// Start 开始答题：WAITING -> RUNNING 并发布第一题
func (e *Engine) Start(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.quiz.Status != model.QuizStatusWaiting {
		return ErrNotWaiting
	}
	if len(rt.questions) == 0 {
		return errors.New("没有题目，无法开始")
	}
	now := time.Now()
	rt.quiz.Status = model.QuizStatusAnswering
	rt.quiz.StartedAt = &now
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Updates(map[string]any{
		"status": model.QuizStatusAnswering, "started_at": now,
	})
	e.Hub.Broadcast(quizID, ws.EventActivityStart, nil)
	return e.publishLocked(rt, 0)
}

// Pause 暂停：冻结倒计时
func (e *Engine) Pause(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	switch rt.quiz.Status {
	case model.QuizStatusAnswering:
		// 保留剩余时间
		rt.pausedRemain = rt.timer.Pause().Milliseconds()
		rt.stopTicker()
		rt.quiz.Status = model.QuizStatusPaused
		e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusPaused)
		e.Hub.Broadcast(quizID, ws.EventActivityPause, map[string]int64{"remain_ms": rt.pausedRemain})
		return nil
	case model.QuizStatusPaused:
		return nil // 幂等
	default:
		return ErrNotRunning
	}
}

// Resume 继续：恢复倒计时
func (e *Engine) Resume(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.quiz.Status != model.QuizStatusPaused {
		return ErrNotRunning
	}
	rt.quiz.Status = model.QuizStatusAnswering
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusAnswering)
	remain := rt.pausedRemain
	if remain > 0 && rt.curIndex >= 0 {
		q := rt.questions[rt.curIndex]
		rt.startTimer(time.Duration(remain)*time.Millisecond, func() { e.forceCollect(quizID, q.ID) })
		rt.deadline = time.Now().Add(time.Duration(remain) * time.Millisecond).UnixMilli()
		rt.startTickerLocked(q.ID)
	}
	e.Hub.Broadcast(quizID, ws.EventActivityResume, map[string]int64{"remain_ms": remain})
	return nil
}

// Next 下一题（最后一题时结束）
func (e *Engine) Next(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.quiz.Status == model.QuizStatusWaiting {
		return ErrNotWaiting
	}
	if rt.quiz.Status == model.QuizStatusFinished {
		return ErrAlreadyEnded
	}
	next := rt.curIndex + 1
	if next >= len(rt.questions) {
		rt.mu.Unlock()
		err := e.End(quizID)
		rt.mu.Lock()
		return err
	}
	return e.publishLocked(rt, next)
}

// Previous 上一题（仅回看，提交已锁定）
func (e *Engine) Previous(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.quiz.Status == model.QuizStatusWaiting || rt.quiz.Status == model.QuizStatusFinished {
		return ErrNotRunning
	}
	prev := rt.curIndex - 1
	if prev < 0 {
		return errors.New("已经是第一题")
	}
	return e.publishLocked(rt, prev)
}

// Reveal 公布答案
func (e *Engine) Reveal(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) {
		return ErrNotRunning
	}
	q := rt.questions[rt.curIndex]
	rt.stopTimer()
	rt.stopTicker()
	rt.deadline = 0
	rt.quiz.Status = model.QuizStatusRevealing
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusRevealing)

	stats, dist := e.questionStats(quizID, q.ID)

	// 用户端（按配置裁剪答案/解析）
	userData := &ws.RevealData{
		QuestionID:  q.ID,
		Stats:       stats,
	}
	if rt.quiz.ShowAnswer {
		userData.CorrectAns = q.Answer
		if rt.quiz.ShowAnalysis {
			userData.Analysis = q.Analysis
		}
	}
	e.Hub.BroadcastUsers(quizID, ws.EventAnswerReveal, userData)

	// 管理端：始终完整（含分布）
	adminData := &ws.RevealData{
		QuestionID:   q.ID,
		CorrectAns:   q.Answer,
		Analysis:     q.Analysis,
		Stats:        stats,
		Distribution: dist,
	}
	e.Hub.Broadcast(quizID, ws.EventAnswerReveal, adminData)
	return nil
}

// End 结束答题
func (e *Engine) End(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	if rt.quiz.Status == model.QuizStatusFinished {
		rt.mu.Unlock()
		return nil
	}
	rt.stopTimer()
	rt.stopTicker()
	rt.deadline = 0
	now := time.Now()
	rt.quiz.Status = model.QuizStatusFinished
	rt.quiz.EndedAt = &now
	rt.mu.Unlock()

	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Updates(map[string]any{
		"status": model.QuizStatusFinished, "ended_at": now,
	})
	e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).Update("finished_at", now)

	rank := e.buildRanking(quizID, 200)
	payload := map[string]any{"ranking": rank}
	e.Hub.Broadcast(quizID, ws.EventActivityEnd, payload)
	return nil
}

// ---------- 发布题目 ----------

// publishLocked 发布题目（调用方持有 rt.mu）
func (e *Engine) publishLocked(rt *Runtime, idx int) error {
	quizID := rt.quiz.ID
	q := rt.questions[idx]

	// 停掉旧计时
	rt.stopTimer()
	rt.stopTicker()
	rt.curIndex = idx
	rt.deadline = 0

	if rt.quiz.Status != model.QuizStatusFinished {
		rt.quiz.Status = model.QuizStatusAnswering
		e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusAnswering)
	}

	// 倒计时
	limit := q.TimeLimit
	if limit <= 0 {
		limit = rt.quiz.PerQuestionTime
	}
	var deadline int64
	if limit > 0 {
		d := time.Duration(limit) * time.Second
		deadline = time.Now().Add(d).UnixMilli()
		rt.deadline = deadline
		rt.startTimer(d, func() { e.forceCollect(quizID, q.ID) })
		rt.startTickerLocked(q.ID)
	}

	// 广播题目（剥离答案）
	opts := rt.getOptionsLocked(e, q.ID)
	brief := &ws.QuestionBrief{
		ID:        q.ID,
		Index:     idx + 1,
		Total:     len(rt.questions),
		Type:      q.Type,
		Content:   q.Content,
		Score:     q.Score,
		Required:  q.Required,
		TimeLimit: limit,
	}
	for _, o := range opts {
		brief.Options = append(brief.Options, ws.Option{Label: o.Label, Content: o.Content})
	}
	e.Hub.Broadcast(quizID, ws.EventQuestionPublish, map[string]any{
		"question":    brief,
		"deadline_at": deadline,
		"server_now":  nowMilli(),
		"status":      rt.quiz.Status,
	})
	return nil
}

// forceCollect 到时强制收卷：必答题记录未答，非必答跳过
func (e *Engine) forceCollect(quizID int64, questionID int64) {
	rt, err := e.Get(quizID)
	if err != nil {
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// 幂等：只处理当前题
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) || rt.questions[rt.curIndex].ID != questionID {
		return
	}
	if rt.quiz.Status != model.QuizStatusAnswering {
		return
	}
	q := rt.questions[rt.curIndex]
	rt.stopTicker()
	rt.deadline = 0

	// 必答题：给未提交者记未答（事务批插，唯一索引幂等）
	if q.Required {
		var pids []model.Participant
		e.DB.Select("user_id").Where("quiz_id = ?", quizID).Find(&pids)
		if len(pids) > 0 {
			uids := make([]int64, len(pids))
			for i, p := range pids {
				uids[i] = p.UserID
			}
			var answered []int64
			e.DB.Model(&model.Answer{}).
				Where("quiz_id = ? AND question_id = ? AND user_id IN ?", quizID, questionID, uids).
				Pluck("user_id", &answered)
			ansSet := map[int64]bool{}
			for _, u := range answered {
				ansSet[u] = true
			}
			unanswered := []int64{}
			for _, u := range uids {
				if !ansSet[u] {
					unanswered = append(unanswered, u)
				}
			}
			if len(unanswered) > 0 {
				records := make([]model.Answer, len(unanswered))
				for i, u := range unanswered {
					records[i] = model.Answer{
						QuizID: quizID, QuestionID: questionID, UserID: u,
						Answer: AnswerUnanswered, IsCorrect: false, Score: 0, Duration: 0, SubmittedAt: time.Now(),
					}
				}
				_ = e.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
			}
		}
	}

	e.Hub.Broadcast(quizID, ws.EventQuestionCountd, &ws.CountdownData{
		QuestionID: questionID, RemainSec: 0, DeadlineAt: nowMilli(),
	})
	e.broadcastStatistics(quizID, questionID)
}

// ---------- 判分 ----------

// SubmitAnswer 提交答案（含防重复、判分、计分、广播）
func (e *Engine) SubmitAnswer(quizID, questionID, userID int64, answer string, durationMs int) (*ws.AnswerResultData, error) {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.quiz.Status == model.QuizStatusFinished {
		return nil, ErrAlreadyEnded
	}
	if rt.quiz.Status != model.QuizStatusAnswering && rt.quiz.Status != model.QuizStatusRevealing {
		return nil, errors.New("当前不可作答")
	}
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) || rt.questions[rt.curIndex].ID != questionID {
		return nil, errors.New("当前题目不匹配")
	}
	q := rt.questions[rt.curIndex]

	// 倒计时已过（宽限 1.5s 网络延迟）
	if rt.deadline > 0 && nowMilli() > rt.deadline+1500 {
		return nil, errors.New("答题时间已到")
	}
	if answer == "" {
		return nil, errors.New("答案不能为空")
	}

	// 规范化多选答案并校验选项合法
	userAns, ok := normalizeAnswer(q.Type, answer, rt.getOptionsLocked(e, q.ID))
	if !ok {
		return nil, errors.New("答案选项不合法")
	}

	// 防重复提交（DB 唯一索引兜底，先查）
	var cnt int64
	e.DB.Model(&model.Answer{}).
		Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, questionID, userID).
		Count(&cnt)
	if cnt > 0 {
		return nil, errors.New("已提交过本题答案")
	}

	isCorrect := userAns == q.Answer
	score := 0
	if isCorrect {
		score = q.Score
	}

	rec := model.Answer{
		QuizID: quizID, QuestionID: questionID, UserID: userID,
		Answer: userAns, IsCorrect: isCorrect, Score: score, Duration: durationMs,
		SubmittedAt: time.Now(),
	}
	if err := e.DB.Create(&rec).Error; err != nil {
		return nil, errors.New("重复提交")
	}

	// 更新参与者累计分（原子）
	if isCorrect {
		e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).
			Updates(map[string]any{
				"score":          gorm.Expr("score + ?", score),
				"correct_count":  gorm.Expr("correct_count + 1"),
			})
	} else {
		e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).
			Update("wrong_count", gorm.Expr("wrong_count + 1"))
	}

	var p model.Participant
	e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p)

	result := &ws.AnswerResultData{
		QuestionID: questionID,
		Answer:     userAns,
		IsCorrect:  isCorrect,
		Score:      score,
		TotalScore: p.Score,
		Revealed:   false, // 即时只通知对错与得分；答案在 reveal 时下发
	}
	e.Hub.SendToUser(quizID, userID, ws.EventAnswerResult, result)

	// 实时统计（管理端）与排行榜
	e.broadcastStatistics(quizID, questionID)
	if rt.quiz.ShowRanking {
		e.Hub.Broadcast(quizID, ws.EventRankingUpdate, &ws.RankingData{Items: e.buildRanking(quizID, 50)})
	}
	return result, nil
}

// normalizeAnswer 多选排序去重、校验选项
func normalizeAnswer(qType, answer string, opts []model.QuestionOption) (string, bool) {
	valid := map[string]bool{}
	for _, o := range opts {
		valid[o.Label] = true
	}
	switch qType {
	case model.QuestionTypeSingle, model.QuestionTypeJudge:
		if len(answer) == 1 && valid[answer] {
			return answer, true
		}
		return "", false
	case model.QuestionTypeMultiple:
		seen := map[rune]bool{}
		var out []rune
		for _, ch := range answer {
			if !valid[string(ch)] || seen[ch] {
				return "", false
			}
			seen[ch] = true
			out = append(out, ch)
		}
		if len(out) == 0 {
			return "", false
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return string(out), true
	}
	return "", false
}

// ---------- 统计与排行 ----------

type QuestionStats struct {
	Participants int            `json:"participants"` // 参与人数
	Answered     int            `json:"answered"`     // 已答
	Correct      int            `json:"correct"`
	Wrong        int            `json:"wrong"`
	MaxScore     int            `json:"max_score"`
	Distribution map[string]int `json:"distribution"` // 选项分布
}

// questionStats 当前题统计（调用方持锁）
func (e *Engine) questionStats(quizID, questionID int64) (*ws.RevealStats, map[string]int) {
	var participants, answered, correct int64
	e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).Count(&participants)
	e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND question_id = ?", quizID, questionID).Count(&answered)
	e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND question_id = ? AND is_correct = ?", quizID, questionID, true).Count(&correct)

	dist := map[string]int{}
	type row struct {
		Answer string
		Cnt    int
	}
	var rows []row
	e.DB.Model(&model.Answer{}).Select("answer, COUNT(*) as cnt").
		Where("quiz_id = ? AND question_id = ? AND answer != ?", quizID, questionID, AnswerUnanswered).
		Group("answer").Scan(&rows)
	for _, r := range rows {
		dist[r.Answer] = r.Cnt
	}
	return &ws.RevealStats{Total: int(answered), Correct: int(correct), Wrong: int(answered) - int(correct)}, dist
}

// broadcastStatistics 实时统计（管理端）
func (e *Engine) broadcastStatistics(quizID, questionID int64) {
	stats, dist := e.questionStats(quizID, questionID)
	var maxScore int
	e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).
		Select("COALESCE(MAX(score),0)").Scan(&maxScore)
	e.Hub.Broadcast(quizID, ws.EventStatisticsUpdate, map[string]any{
		"question_id":  questionID,
		"participants": stats.Total,
		"participants_all": func() int64 {
			var n int64
			e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).Count(&n)
			return n
		}(),
		"answered":     stats.Total,
		"correct":      stats.Correct,
		"wrong":        stats.Wrong,
		"max_score":    maxScore,
		"distribution": dist,
	})
}

// buildRanking 排行榜（分数 desc、正确数 desc、加入时间 asc）
func (e *Engine) buildRanking(quizID int64, limit int) []ws.RankingItem {
	type row struct {
		UserID       int64
		Nickname     string
		Score        int
		CorrectCount int
	}
	var rows []row
	e.DB.Table("participants").
		Select("participants.user_id, users.nickname, participants.score, participants.correct_count").
		Joins("JOIN users ON users.id = participants.user_id").
		Where("participants.quiz_id = ?", quizID).
		Order("participants.score DESC, participants.correct_count DESC, participants.joined_at ASC").
		Limit(limit).Scan(&rows)

	items := make([]ws.RankingItem, len(rows))
	for i, r := range rows {
		items[i] = ws.RankingItem{
			Rank: i + 1, UserID: r.UserID, Nickname: r.Nickname,
			Score: r.Score, Correct: r.CorrectCount,
		}
	}
	return items
}

// ---------- Runtime 计时器辅助（持锁调用） ----------

func (rt *Runtime) startTimer(d time.Duration, fn func()) {
	rt.timer = NewSyncTimer(d, fn)
}

func (rt *Runtime) stopTimer() {
	if rt.timer != nil {
		rt.timer.Cancel()
		rt.timer = nil
	}
}

func (rt *Runtime) startTickerLocked(questionID int64) {
	rt.stopTicker()
	e := rt.engine
	if e == nil {
		return
	}
	qid := rt.quiz.ID
	rt.tick = startTicker(func() {
		rt.mu.Lock()
		dl := rt.deadline
		rt.mu.Unlock()
		if dl <= 0 {
			return
		}
		remain := (dl - nowMilli()) / 1000
		if remain < 0 {
			remain = 0
		}
		e.Hub.Broadcast(qid, ws.EventQuestionCountd, &ws.CountdownData{
			QuestionID: questionID, RemainSec: int(remain), DeadlineAt: dl,
		})
	})
}

func (rt *Runtime) stopTicker() {
	if rt.tick != nil {
		rt.tick.Stop()
		rt.tick = nil
	}
}
