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

	// 当前题 + 抢答状态恢复（进行中或已结束均返回获答名单与本人抢答结果）
	if rt.curIndex >= 0 && rt.curIndex < len(rt.questions) {
		cq := rt.questions[rt.curIndex]
		opts := rt.getOptionsLocked(e, cq.ID)
		brief := &ws.QuestionBrief{
			ID:        cq.ID,
			Index:     rt.curIndex + 1,
			Total:     len(rt.questions),
			Type:      cq.Type,
			Content:   cq.Content,
			Score:     cq.Score,
			Required:  cq.Required,
			TimeLimit: cq.TimeLimit,
		}
		for _, o := range opts {
			brief.Options = append(brief.Options, ws.Option{Label: o.Label, Content: o.Content})
		}
		data.Question = brief

		if rt.quiz.Status == model.QuizStatusRushing {
			data.DeadlineAt = rt.rushDeadline
			data.RushOpenAt = rt.rushOpenAt
			data.RushWinners = e.redisWinners(rt, cq.ID)
			if claims.Role == auth.RoleUser {
				data.MyRushRank = e.myRushRank(quizID, cq.ID, claims.UserID)
			}
		} else if rt.quiz.Status == model.QuizStatusRevealing {
			// 刷新恢复：公布阶段下发答案分布
			var rows []struct{ Answer string; Cnt int; Score int }
			e.DB.Model(&model.Answer{}).Select("answer, COUNT(*) as cnt, MAX(score) as score").
				Where("quiz_id = ? AND question_id = ?", quizID, cq.ID).
				Group("answer").Scan(&rows)
			data.Distribution, data.AnswerScores = map[string]int{}, map[string]int{}
			for _, r := range rows {
				data.Distribution[r.Answer] = r.Cnt
				data.AnswerScores[r.Answer] = r.Score
			}
			if e.isRushQuestion(quizID, cq.ID) {
				data.RushWinners = e.rushWinnersFromDB(quizID, cq.ID)
			}
		} else if e.isRushQuestion(quizID, cq.ID) {
			data.RushWinners = e.rushWinnersFromDB(quizID, cq.ID)
			if claims.Role == auth.RoleUser {
				var rec model.RushRecord
				if e.DB.Where("quiz_id = ? AND question_id = ? AND user_id = ?", quizID, cq.ID, claims.UserID).
					First(&rec); rec.Rank > 0 {
					data.MyRushRank = rec.Rank
				}
			}
		}
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
