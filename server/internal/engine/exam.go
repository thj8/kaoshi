package engine

import (
	"errors"
	"log"
	"sort"
	"time"

	"kaoshi/internal/model"
	"kaoshi/internal/ws"
)

// 考试（自由切题）模式：
//   - 管理员开始后全卷一次性下发（试卷接口，答案绝不下发），统一时长（quiz.TotalTime，0=不限）
//   - 用户自由前后切题，选择即保存（服务端 upsert，可反复修改，直到交卷/到时）
//   - 交卷或到时统一收卷：从 answers 重算参与者成绩（考试期间 participants.score 不实时更新，
//     逐题对错/得分不回传，防止反复试答猜答案）
//   - 与逐题模式的差异点：answers 记录为「可变草稿」，唯一索引仍保证 (quiz,question,user) 一条，
//     交卷后从全部记录一次性定分（领域不变量「分数只在首次提交累加」仅约束逐题/抢答模式）

// startExamLocked 开始考试：WAITING -> RUNNING，不发布单题（调用方持有 rt.mu）
func (e *Engine) startExamLocked(rt *Runtime) error {
	quizID := rt.quiz.ID
	now := time.Now()
	rt.quiz.Status = model.QuizStatusRunning
	rt.quiz.StartedAt = &now

	var deadline int64
	if rt.quiz.TotalTime > 0 {
		deadline = now.Add(time.Duration(rt.quiz.TotalTime) * time.Second).UnixMilli()
		rt.deadline = deadline
	}
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Updates(map[string]any{
		"status": model.QuizStatusRunning, "started_at": now,
	})

	e.Hub.Broadcast(quizID, ws.EventActivityStart, map[string]any{
		"deadline_at": deadline,
		"remain_sec":  rt.quiz.TotalTime,
	})
	log.Printf("[ctrl] quiz=%d 开始考试（共%d题，时长%ds）", quizID, len(rt.questions), rt.quiz.TotalTime)

	if deadline > 0 {
		rt.startTimer(time.Duration(rt.quiz.TotalTime)*time.Second, func() { e.End(quizID) })
		// 每秒广播考试整体倒计时（QuestionID=0 表示考试级，非单题）
		rt.startTickerLocked(0)
	}
	return nil
}

// examCtx 考试模式通用校验（调用方持有 rt.mu）
func examCtx(rt *Runtime) error {
	if rt.quiz.Mode != model.ModeExam {
		return errors.New("非考试模式")
	}
	if rt.quiz.Status == model.QuizStatusWaiting {
		return errors.New("考试尚未开始")
	}
	if rt.quiz.Status == model.QuizStatusFinished {
		return ErrAlreadyEnded
	}
	if rt.deadline > 0 && nowMilli() > rt.deadline {
		return errors.New("考试时间已到")
	}
	return nil
}

// SavePaperAnswer 保存/更新试卷答案（可反复修改，直到交卷/到时）
func (e *Engine) SavePaperAnswer(quizID, userID, questionID int64, answer string, durationMs int) (map[string]any, error) {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if err := examCtx(rt); err != nil {
		return nil, err
	}
	// 已交卷者锁定
	var p model.Participant
	if err := e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p).Error; err == nil && p.FinishedAt != nil {
		return nil, errors.New("已交卷，不能再修改答案")
	}

	// 题目必须属于本场
	var q *model.Question
	qIdx := -1
	for i := range rt.questions {
		if rt.questions[i].ID == questionID {
			q = &rt.questions[i]
			qIdx = i
			break
		}
	}
	if q == nil {
		return nil, ErrNotFound
	}

	// 空答案 = 清除草稿（多选逐一取消后允许全空；清除后该题视为未答）
	if answer == "" {
		e.DB.Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, questionID, userID).
			Delete(&model.Answer{})
		var answered int64
		e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).Count(&answered)
		var cu model.User
		e.DB.First(&cu, userID)
		log.Printf("[answer] quiz=%d 考试第%d题 %s 清除答案草稿", quizID, qIdx+1, cu.Nickname)
		return map[string]any{"answered": answered, "total": len(rt.questions)}, nil
	}

	// 答案合法性：选项 label 白名单 + 去重 + 排序（多选存 "ABC"）
	opts := rt.getOptionsLocked(e, questionID)
	labels := map[string]bool{}
	for _, o := range opts {
		labels[o.Label] = true
	}
	seen := map[string]bool{}
	clean := ""
	for _, r := range answer {
		ch := string(r)
		if !labels[ch] {
			return nil, errors.New("答案包含非法选项")
		}
		if !seen[ch] {
			seen[ch] = true
			clean += ch
		}
	}
	if clean == "" {
		return nil, errors.New("请选择答案")
	}
	runes := []rune(clean)
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })
	clean = string(runes)

	// 服务端判分（仅入库，不回传对错）
	isCorrect := clean == q.Answer
	score := 0
	if isCorrect {
		score = rt.quiz.TypeScore(q.Type, true)
		if score <= 0 {
			score = q.Score
		}
	}

	now := time.Now()
	act := "答"
	var rec model.Answer
	if err := e.DB.Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, questionID, userID).
		First(&rec).Error; err == nil {
		// 已有记录 → 更新（考试模式答案可改）
		act = "改答"
		rec.Answer = clean
		rec.IsCorrect = isCorrect
		rec.Score = score
		rec.Duration = durationMs
		rec.SubmittedAt = now
		if err := e.DB.Save(&rec).Error; err != nil {
			return nil, errors.New("保存失败，请重试")
		}
	} else {
		// GORM 零值陷阱：SubmittedAt 必须显式赋值
		rec = model.Answer{
			QuizID: quizID, QuestionID: questionID, UserID: userID,
			Answer: clean, IsCorrect: isCorrect, Score: score, Duration: durationMs,
			SubmittedAt: now,
		}
		if err := e.DB.Create(&rec).Error; err != nil {
			// 并发竞态（唯一索引冲突）→ 退化为更新
			if err := e.DB.Model(&model.Answer{}).
				Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, questionID, userID).
				Updates(map[string]any{
					"answer": clean, "is_correct": isCorrect, "score": score,
					"duration": durationMs, "submitted_at": now,
				}).Error; err != nil {
				return nil, errors.New("保存失败，请重试")
			}
		}
	}

	// 答题提交日志（服务端留痕，页面上不展示；与普通模式 [answer] 同口径）
	var u model.User
	e.DB.First(&u, userID)
	log.Printf("[answer] quiz=%d 考试第%d/%d题 %s %s「%s」题目「%s」正确答案=%s 得分=%d",
		quizID, qIdx+1, len(rt.questions), u.Nickname, act, clean, clip(q.Content, 15), q.Answer, score)

	var answered int64
	e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).Count(&answered)
	return map[string]any{"answered": answered, "total": len(rt.questions)}, nil
}

// FinalizePaper 交卷：重算本人成绩并锁定（幂等，重复交卷返回同一汇总）
func (e *Engine) FinalizePaper(quizID, userID int64) (map[string]any, error) {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.quiz.Mode != model.ModeExam {
		return nil, errors.New("非考试模式")
	}
	if rt.quiz.Status == model.QuizStatusWaiting {
		return nil, errors.New("考试尚未开始")
	}

	var p model.Participant
	finalized := false
	if err := e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p).Error; err != nil {
		return nil, errors.New("参赛信息不存在，请重新加入本场考试")
	}

	if p.FinishedAt == nil {
		if rt.quiz.Status == model.QuizStatusRunning && rt.deadline > 0 && nowMilli() > rt.deadline {
			return nil, errors.New("考试时间已到")
		}
		// 从答题记录重算本人成绩（终局定分）
		e.examRecomputeParticipant(quizID, userID)
		now := time.Now()
		e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).
			Update("finished_at", now)
		e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p)
		finalized = true
	}

	var totalQ int64
	e.DB.Model(&model.Question{}).Where("quiz_id = ?", quizID).Count(&totalQ)
	var answered, correct int64
	e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).Count(&answered)
	e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ? AND is_correct = ?", quizID, userID, true).Count(&correct)

	rk := e.UserRank(quizID, userID)
	if finalized {
		var u model.User
		e.DB.First(&u, userID)
		log.Printf("[exam] quiz=%d %s 交卷：答%d/%d题 对%d 错%d 得分%d 排名第%d",
			quizID, u.Nickname, answered, totalQ, correct, answered-correct, p.Score, rk)
	}

	return map[string]any{
		"score":    p.Score,
		"answered": answered,
		"total":    totalQ,
		"correct":  correct,
		"wrong":    answered - correct,
		"rank":     rk,
		"finished": rt.quiz.Status == model.QuizStatusFinished,
	}, nil
}

// examRecomputeParticipants 从答题记录重算全部参与者成绩（收卷时调用）
func (e *Engine) examRecomputeParticipants(quizID int64) {
	var uids []int64
	e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).Pluck("user_id", &uids)
	for _, uid := range uids {
		e.examRecomputeParticipant(quizID, uid)
	}
}

// examRecomputeParticipant 从 answers 表重算单位参与者累计分/对错数
func (e *Engine) examRecomputeParticipant(quizID, userID int64) {
	var row struct {
		Score   int
		Correct int64
		Count   int64
	}
	e.DB.Model(&model.Answer{}).
		Select("COALESCE(SUM(score),0) as score, SUM(is_correct) as correct, COUNT(*) as count").
		Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Scan(&row)
	e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Updates(map[string]any{
			"score":         row.Score,
			"correct_count": int(row.Correct),
			"wrong_count":   int(row.Count - row.Correct),
		})
}
