package engine

import (
	"sort"
	"time"

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
	Index       int     `json:"index"`
	QuestionID  int64   `json:"question_id"`
	Type        string  `json:"type"`
	Content     string  `json:"content"`
	Answered    int     `json:"answered"`
	Correct     int     `json:"correct"`
	Wrong       int     `json:"wrong"`
	CorrectRate float64 `json:"correct_rate"` // 百分比
	AvgDuration float64 `json:"avg_duration"` // 毫秒
}

// RankRow 排名行
type RankRow struct {
	Rank        int    `json:"rank"`
	UserID      int64  `json:"user_id"`
	Nickname    string `json:"nickname"`
	Score       int    `json:"score"`
	Correct     int    `json:"correct"`
	Wrong       int    `json:"wrong"`
	SubmittedAt int64  `json:"submitted_at"` // 交卷时间（unix毫秒；0=未交卷，考试模式返回）
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
	if quiz.Mode == model.ModeExam {
		e.examRanking(quizID, s)
		return s
	}
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

// examRanking 考试（自由切题）模式实时排名：
//   - 分数直接从 answers 草稿聚合（保存时服务端已判分，participants.score 交卷才终局化）→ 考试中也能实时看排名
//   - 最高/最低/平均分、平均正确率同步改为实时口径，与大屏榜单一致
//   - 同分排序：以交卷时间为准（早者在前）；未交卷者用最后一次保存时间作为「先到分」代理
func (e *Engine) examRanking(quizID int64, s *OverallStats) {
	var rows []struct {
		UserID     int64
		Nickname   string
		Score      int
		Correct    int64
		Answered   int64
		JoinedAt   time.Time
		FinishedAt *time.Time
		LastSave   *time.Time
	}
	e.DB.Table("participants p").
		Select("p.user_id, u.nickname, COALESCE(SUM(a.score),0) as score, COALESCE(SUM(a.is_correct),0) as correct, "+
			"COUNT(a.id) as answered, p.joined_at, p.finished_at, MAX(a.submitted_at) as last_save").
		Joins("JOIN users u ON u.id = p.user_id").
		Joins("LEFT JOIN answers a ON a.quiz_id = p.quiz_id AND a.user_id = p.user_id AND a.answer != ?", AnswerUnanswered).
		Where("p.quiz_id = ?", quizID).
		Group("p.user_id, u.nickname, p.joined_at, p.finished_at").
		Limit(200).Scan(&rows)

	tieTime := func(i int) time.Time {
		if rows[i].FinishedAt != nil {
			return *rows[i].FinishedAt
		}
		if rows[i].LastSave != nil {
			return *rows[i].LastSave
		}
		return rows[i].JoinedAt
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		di, dj := rows[i].FinishedAt != nil, rows[j].FinishedAt != nil
		if di != dj {
			return di // 同分：已交卷者排前
		}
		return tieTime(i).Before(tieTime(j)) // 都交卷：交卷早者在前；都未交：先到分者在前
	})

	// 实时口径的整场汇总（与榜单同源，避免「最高分0但榜首80分」的不一致）
	totalScore, answeredTotal, correctTotal := 0, int64(0), int64(0)
	minScore := -1
	for i, r := range rows {
		totalScore += r.Score
		answeredTotal += r.Answered
		correctTotal += r.Correct
		if r.Score > s.MaxScore {
			s.MaxScore = r.Score
		}
		if minScore < 0 || r.Score < minScore {
			minScore = r.Score
		}
		s.Ranking = append(s.Ranking, RankRow{
			Rank: i + 1, UserID: r.UserID, Nickname: r.Nickname,
			Score: r.Score, Correct: int(r.Correct), Wrong: int(r.Answered - r.Correct),
		})
		if rows[i].FinishedAt != nil {
			s.Ranking[i].SubmittedAt = rows[i].FinishedAt.UnixMilli()
		}
	}
	if len(rows) > 0 {
		s.MinScore = minScore
		s.AvgScore = float64(totalScore) / float64(len(rows))
	}
	if answeredTotal > 0 {
		s.AvgCorrect = float64(correctTotal) / float64(answeredTotal) * 100
	}
}
