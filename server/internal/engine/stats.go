package engine

import (
	"kaoshi/internal/model"
)

// OverallStats 整场统计（管理端）
type OverallStats struct {
	Status       string         `json:"status"`
	Participants int            `json:"participants"`
	Finished     int            `json:"finished"`
	AvgScore     float64        `json:"avg_score"`
	MaxScore     int            `json:"max_score"`
	MinScore     int            `json:"min_score"`
	AvgCorrect   float64        `json:"avg_correct_rate"` // 平均正确率(0-100)
	Questions    []QuestionStat `json:"questions"`
	Ranking      []RankRow      `json:"ranking"`
}

// QuestionStat 题目统计
type QuestionStat struct {
	Index      int     `json:"index"`
	QuestionID int64   `json:"question_id"`
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Answered   int     `json:"answered"`
	Correct    int     `json:"correct"`
	Wrong      int     `json:"wrong"`
	CorrectRate float64 `json:"correct_rate"` // 百分比
	AvgDuration float64 `json:"avg_duration"` // 毫秒
}

// RankRow 排名行
type RankRow struct {
	Rank     int    `json:"rank"`
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Score    int    `json:"score"`
	Correct  int    `json:"correct"`
	Wrong    int    `json:"wrong"`
}

// Statistics 整场统计（实时/结束均可调用）
func (e *Engine) Statistics(quizID int64) *OverallStats {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil
	}
	rt.mu.Lock()
	quiz := rt.quiz
	questions := append([]model.Question(nil), rt.questions...)
	rt.mu.Unlock()

	s := &OverallStats{Status: quiz.Status, Questions: []QuestionStat{}}

	var participants []model.Participant
	e.DB.Where("quiz_id = ?", quizID).Find(&participants)
	s.Participants = len(participants)
	if len(participants) > 0 {
		totalScore, correct, finished := 0, 0, 0
		minScore := participants[0].Score
		for _, p := range participants {
			totalScore += p.Score
			correct += p.CorrectCount
			if p.Score > s.MaxScore {
				s.MaxScore = p.Score
			}
			if p.Score < minScore {
				minScore = p.Score
			}
			if p.FinishedAt != nil {
				finished++
			}
		}
		s.Finished = finished
		s.MinScore = minScore
		s.AvgScore = float64(totalScore) / float64(len(participants))
		var totalAnswered int64 // 排除未答占位行（answer="-"），正确率分母只算真实作答
		e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND answer != ?", quizID, AnswerUnanswered).Count(&totalAnswered)
		if totalAnswered > 0 {
			s.AvgCorrect = float64(correct) / float64(totalAnswered) * 100
		}
	}

	// 题目维度
	for i, q := range questions {
		qs := QuestionStat{
			Index: i + 1, QuestionID: q.ID, Type: q.Type, Content: q.Content,
		}
		var answered, correct int64
		e.DB.Model(&model.Answer{}).
			Where("quiz_id = ? AND question_id = ? AND answer != ?", quizID, q.ID, AnswerUnanswered).Count(&answered)
		e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND question_id = ? AND is_correct = ?", quizID, q.ID, true).Count(&correct)
		qs.Answered = int(answered)
		qs.Correct = int(correct)
		qs.Wrong = int(answered - correct)
		if answered > 0 {
			qs.CorrectRate = float64(correct) / float64(answered) * 100
		}
		var avgDur float64
		e.DB.Model(&model.Answer{}).Where("quiz_id = ? AND question_id = ?", quizID, q.ID).
			Select("COALESCE(AVG(duration),0)").Scan(&avgDur)
		qs.AvgDuration = avgDur
		s.Questions = append(s.Questions, qs)
	}

	// 排名
	var rows []struct {
		UserID       int64
		Nickname     string
		Score        int
		CorrectCount int
		WrongCount   int
	}
	e.DB.Table("participants").
		Select("participants.user_id, users.nickname, participants.score, participants.correct_count, participants.wrong_count").
		Joins("JOIN users ON users.id = participants.user_id").
		Where("participants.quiz_id = ?", quizID).
		Order("participants.score DESC, participants.correct_count DESC, participants.joined_at ASC").
		Limit(200).Scan(&rows)
	s.Ranking = make([]RankRow, len(rows))
	for i, r := range rows {
		s.Ranking[i] = RankRow{Rank: i + 1, UserID: r.UserID, Nickname: r.Nickname, Score: r.Score, Correct: r.CorrectCount, Wrong: r.WrongCount}
	}
	return s
}
