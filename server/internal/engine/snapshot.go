package engine

import (
	"fmt"

	"kaoshi/internal/auth"
	"kaoshi/internal/model"
	"kaoshi/internal/ws"
)

// Snapshot 生成全量状态快照（用于 WS 连接建立/重连恢复）
func (e *Engine) Snapshot(claims *auth.Claims) (*ws.SyncData, error) {
	quizID := claims.QuizID
	if quizID == 0 { // admin sync（quiz 由 ws 层指定后重新调用）
		return nil, fmt.Errorf("no quiz")
	}
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, err
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	var count int64
	e.DB.Model(&model.Participant{}).Where("quiz_id = ?", quizID).Count(&count)

	data := &ws.SyncData{
		Quiz: &ws.QuizBrief{
			ID:          rt.quiz.ID,
			Title:       rt.quiz.Title,
			Description: rt.quiz.Description,
			Mode:        rt.quiz.Mode,
			ShowAnswer:  rt.quiz.ShowAnswer,
			ShowAnalysis: rt.quiz.ShowAnalysis,
			ShowRanking: rt.quiz.ShowRanking,
			ParticipantCount: int(count),
		},
		Status:     rt.quiz.Status,
		DeadlineAt: rt.deadline,
		RushActive: rt.quiz.Status == model.QuizStatusRushing,
		ServerNow:  nowMilli(),
	}

	// 抢答状态恢复（进行中或已结束均返回获答名单与本人抢答结果）
	if rt.curIndex >= 0 && rt.curIndex < len(rt.questions) {
		cq := rt.questions[rt.curIndex]
		if rt.quiz.Status == model.QuizStatusRushing {
			data.DeadlineAt = rt.rushDeadline
			data.RushWinners = e.redisWinners(rt, cq.ID)
			if claims.Role == auth.RoleUser {
				data.MyRushRank = e.myRushRank(quizID, cq.ID, claims.UserID)
			}
		} else if e.isRushQuestion(quizID, cq.ID) {
			data.RushWinners = e.rushWinnersFromDB(quizID, cq.ID, rt.quiz.RushBonusScore)
			if claims.Role == auth.RoleUser {
				var rec model.RushRecord
				if e.DB.Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, cq.ID, claims.UserID).
					First(&rec); rec.Rank > 0 {
					data.MyRushRank = rec.Rank
				}
			}
		}
	}

	// 当前题目（剥离答案/解析）
	if rt.curIndex >= 0 && rt.curIndex < len(rt.questions) {
		q := rt.questions[rt.curIndex]
		opts := rt.getOptionsLocked(e, q.ID)
		brief := &ws.QuestionBrief{
			ID:        q.ID,
			Index:     rt.curIndex + 1,
			Total:     len(rt.questions),
			Type:      q.Type,
			Content:   q.Content,
			Score:     q.Score,
			Required:  q.Required,
			TimeLimit: q.TimeLimit,
		}
		for _, o := range opts {
			brief.Options = append(brief.Options, ws.Option{Label: o.Label, Content: o.Content})
		}
		data.Question = brief
	}

	// 用户本人信息
	if claims.Role == auth.RoleUser {
		me := &ws.MeInfo{UserID: claims.UserID, Nickname: claims.Nick}
		var p model.Participant
		if err := e.DB.Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).First(&p).Error; err == nil {
			me.Score = p.Score
		}
		var answered int64
		e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND user_id = ?", quizID, claims.UserID).Count(&answered)
		me.Answered = int(answered)
		data.Me = me
	}
	return data, nil
}

// SnapshotForQuiz 管理端快照（指定 quizID）
func (e *Engine) SnapshotForQuiz(claims *auth.Claims, quizID int64) (*ws.SyncData, error) {
	c2 := *claims
	c2.QuizID = quizID
	return e.Snapshot(&c2)
}
