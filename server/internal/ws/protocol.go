package ws

import "encoding/json"

// 消息协议：{event, data, ts}
// 事件名对齐 task.md 二十二节

const (
	// 服务器 -> 客户端
	EventSync            = "sync"              // 全量状态同步（重连/刷新恢复）
	EventActivityStart   = "activity:start"
	EventActivityPause   = "activity:pause"
	EventActivityResume  = "activity:resume"
	EventActivityEnd     = "activity:end"
	EventQuestionPublish = "question:publish"  // 发布/切换题目
	EventQuestionCountd  = "question:countdown"// 剩余时间广播
	EventAnswerAccepted  = "answer:accepted"   // 服务端已收到提交
	EventAnswerResult    = "answer:result"     // 个人判分结果（即时/公布时）
	EventAnswerReveal    = "answer:reveal"     // 公布答案
	EventRushStart       = "rush:start"
	EventRushSuccess     = "rush:success"
	EventRushFailed      = "rush:failed"
	EventRushEnd         = "rush:end"
	EventRankingUpdate   = "ranking:update"
	EventStatisticsUpdate= "statistics:update"
	EventError           = "error"

	// 客户端 -> 服务器
	EventPing  = "ping"
	EventPong  = "pong"
)

// Message WS 消息
type Message struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
	TS    int64           `json:"ts,omitempty"` // 服务器毫秒时间戳（下行）
}

// Payloads 事件负载定义
type SyncData struct {
	Quiz       *QuizBrief      `json:"quiz"`
	Status     string          `json:"status"`      // quiz 状态机
	Question   *QuestionBrief  `json:"question"`    // 当前题目（无答案）
	DeadlineAt int64           `json:"deadline_at"` // 当前题截止毫秒时间戳（0=无倒计时）
	RushActive bool            `json:"rush_active"` // 抢答进行中
	Me         *MeInfo         `json:"me"`          // 本人信息（用户连接时）
	ServerNow  int64           `json:"server_now"`  // 服务器当前毫秒时间
}

type QuizBrief struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Mode        string `json:"mode"`
	ShowAnswer  bool   `json:"show_answer"`
	ShowAnalysis bool `json:"show_analysis"`
	ShowRanking bool   `json:"show_ranking"`
	ParticipantCount int `json:"participant_count"`
}

type QuestionBrief struct {
	ID        int64    `json:"id"`
	Index     int      `json:"index"`  // 第几题（1-based）
	Total     int      `json:"total"`  // 总题数
	Type      string   `json:"type"`
	Content   string   `json:"content"`
	Options   []Option `json:"options"`
	Score     int      `json:"score"`
	Required  bool     `json:"required"`
	TimeLimit int      `json:"time_limit"` // 秒
}

type Option struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

type MeInfo struct {
	UserID  int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Score   int    `json:"score"`
	Answered int   `json:"answered"` // 已答题目数
}

type CountdownData struct {
	QuestionID int64 `json:"question_id"`
	RemainSec  int   `json:"remain_sec"`
	DeadlineAt int64 `json:"deadline_at"`
}

type AnswerResultData struct {
	QuestionID int64  `json:"question_id"`
	Answer     string `json:"answer"`      // 用户提交的答案
	IsCorrect  bool   `json:"is_correct"`
	Score      int    `json:"score"`       // 本次得分
	TotalScore int    `json:"total_score"` // 累计分
	CorrectAns string `json:"correct_answer,omitempty"` // 仅公布时下发
	Analysis   string `json:"analysis,omitempty"`
	Revealed   bool   `json:"revealed"`
}

type RevealData struct {
	QuestionID  int64         `json:"question_id"`
	CorrectAns  string        `json:"correct_answer"`
	Analysis    string        `json:"analysis,omitempty"`
	Distribution map[string]int `json:"distribution,omitempty"` // 选项分布（管理端用）
	Stats       *RevealStats `json:"stats,omitempty"`
}

type RevealStats struct {
	Total   int `json:"total"`   // 已答
	Correct int `json:"correct"` // 正确
	Wrong   int `json:"wrong"`
}

type RankingItem struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"user_id"`
	Nickname  string `json:"nickname"`
	Score     int    `json:"score"`
	Correct   int    `json:"correct"`
}

type RankingData struct {
	Items []RankingItem `json:"items"`
}

type RushStartData struct {
	QuestionID int64 `json:"question_id"`
	Winners    int   `json:"winners"`  // 名额
	DeadlineAt int64 `json:"deadline_at"` // 抢答截止毫秒
}

type RushResultData struct {
	QuestionID int64  `json:"question_id"`
	Rank       int    `json:"rank"`
	Nickname   string `json:"nickname"`
}

type ErrorData struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
