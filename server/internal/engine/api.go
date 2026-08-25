package engine

import (
	"kaoshi/internal/ws"
)

// Ranking 公开排行榜（含全量行）
func (e *Engine) Ranking(quizID int64, limit int) []ws.RankingItem {
	return e.buildRanking(quizID, limit)
}

// UserRank 某用户排名（1-based）
func (e *Engine) UserRank(quizID, userID int64) int {
	items := e.buildRanking(quizID, 1000)
	for _, it := range items {
		if it.UserID == userID {
			return it.Rank
		}
	}
	return 0
}

// CurrentQuestionInfo 当前题公开信息（REST 兜底）
func (e *Engine) CurrentQuestionInfo(quizID int64) *ws.QuestionBrief {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) {
		return nil
	}
	q := rt.questions[rt.curIndex]
	opts := rt.GetOptions(e, q.ID)
	brief := &ws.QuestionBrief{
		ID: q.ID, Index: rt.curIndex + 1, Total: len(rt.questions),
		Type: q.Type, Content: q.Content, Score: q.Score,
		Required: q.Required, TimeLimit: q.TimeLimit,
	}
	for _, o := range opts {
		brief.Options = append(brief.Options, ws.Option{Label: o.Label, Content: o.Content})
	}
	return brief
}
