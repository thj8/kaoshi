package model

import "time"

// 用户（答题者，账号密码登录）
type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:128" json:"-"` // bcrypt，绝不下发
	Nickname     string    `gorm:"size:64;notNull;index" json:"nickname"`
	CreatedAt    time.Time `json:"created_at"`
}

// 答题状态（服务端状态机，客户端只读）
const (
	QuizStatusWaiting   = "WAITING"   // 等待开始
	QuizStatusRunning   = "RUNNING"   // 进行中（普通答题）
	QuizStatusPaused    = "PAUSED"    // 暂停
	QuizStatusRushing   = "RUSHING"   // 抢答中
	QuizStatusAnswering = "ANSWERING" // 抢答成功者答题中
	QuizStatusRevealing = "REVEALING" // 公布答案
	QuizStatusFinished  = "FINISHED"  // 已结束
)

// 答题模式
const (
	ModeNormal = "normal" // 普通模式：全员逐题作答
	ModeRush   = "rush"   // 抢答模式
)

// 答题活动
type Quiz struct {
	ID          int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string `gorm:"size:128;notNull" json:"title"`
	Description string `gorm:"size:1024" json:"description"`
	Status      string `gorm:"size:16;notNull;default:WAITING;index" json:"status"`
	Mode        string `gorm:"size:16;notNull;default:normal" json:"mode"`

	// 全局配置（bool 不加 gorm default 标签：GORM 对零值+default 会改写字段，导致显式 false 失效）
	TotalTime       int  `gorm:"notNull" json:"total_time"`        // 总答题时间（秒，0=不限）
	PerQuestionTime int  `gorm:"notNull;default:30" json:"per_question_time"` // 每题默认答题时间（秒）
	RushEnabled     bool `gorm:"notNull" json:"rush_enabled"`      // 是否开启抢答
	ShowAnswer      bool `gorm:"notNull" json:"show_answer"`       // 是否显示正确答案
	ShowAnalysis    bool `gorm:"notNull" json:"show_analysis"`     // 是否显示解析
	ShowRanking     bool `gorm:"notNull" json:"show_ranking"`      // 是否显示排行榜

	// 抢答配置
	RushWinnerCount int `gorm:"notNull;default:1" json:"rush_winner_count"` // 每题抢答名额
	RushTime        int `gorm:"notNull;default:10" json:"rush_time"`        // 抢答窗口（秒）
	RushAnswerTime  int `gorm:"notNull;default:20" json:"rush_answer_time"` // 抢答成功后答题时间（秒）
	RushBonusScore  int `gorm:"notNull;default:5" json:"rush_bonus_score"`  // 抢答成功奖励分
	RushWrongScore  int `gorm:"notNull;default:0" json:"rush_wrong_score"`  // 抢答题答错是否扣分（>0 开启，扣本题对应分值）

	CreatedAt time.Time  `json:"created_at"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`

	// 关联（不入库，仅查询用）
	Questions []Question `gorm:"foreignKey:QuizID" json:"questions,omitempty"`
}

// 题型
const (
	QuestionTypeSingle   = "single"   // 单选
	QuestionTypeMultiple = "multiple" // 多选
	QuestionTypeJudge    = "judge"    // 判断
)

// 题目（answer 存储：单选 "B"；多选 "ABC"（按字母排序）；判断 "A"=正确 "B"=错误）
type Question struct {
	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	QuizID    int64  `gorm:"notNull;index:idx_quiz_sort" json:"quiz_id"`
	Type      string `gorm:"size:16;notNull" json:"type"`
	Content   string `gorm:"size:1024;notNull" json:"content"`
	Answer    string `gorm:"size:16;notNull" json:"-"` // 序列化时剥离，绝不下发
	Analysis  string `gorm:"size:1024" json:"-"`
	Score     int    `gorm:"notNull;default:10" json:"score"`
	Required  bool   `gorm:"notNull" json:"required"`
	Sort      int    `gorm:"notNull;index:idx_quiz_sort" json:"sort"`
	TimeLimit int    `gorm:"notNull;default:0" json:"time_limit"` // 本题专属倒计时（秒），0=用 quiz 全局配置

	CreatedAt time.Time `json:"created_at"`
}

// 题目选项
type QuestionOption struct {
	ID         int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	QuestionID int64  `gorm:"notNull;index" json:"question_id"`
	Label      string `gorm:"size:8;notNull" json:"label"` // A/B/C/D
	Content    string `gorm:"size:256;notNull" json:"content"`
	Sort       int    `gorm:"notNull" json:"sort"`
}

// 参与者（一场活动的成绩主体）
type Participant struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	QuizID       int64      `gorm:"notNull;uniqueIndex:idx_quiz_user" json:"quiz_id"`
	UserID       int64      `gorm:"notNull;uniqueIndex:idx_quiz_user" json:"user_id"`
	Score        int        `gorm:"notNull;default:0" json:"score"`
	CorrectCount int        `gorm:"notNull;default:0" json:"correct_count"`
	WrongCount   int        `gorm:"notNull;default:0" json:"wrong_count"`
	JoinedAt     time.Time  `json:"joined_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

// 答题记录（quiz+question+user 唯一，防重复提交/计分）
type Answer struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	QuizID      int64     `gorm:"notNull;uniqueIndex:idx_ans" json:"quiz_id"`
	QuestionID  int64     `gorm:"notNull;uniqueIndex:idx_ans" json:"question_id"`
	UserID      int64     `gorm:"notNull;uniqueIndex:idx_ans" json:"user_id"`
	Answer      string    `gorm:"size:16;notNull" json:"answer"`
	IsCorrect   bool      `json:"is_correct"`
	Score       int       `json:"score"`
	Duration    int       `json:"duration"` // 用时（毫秒）
	SubmittedAt time.Time `json:"submitted_at"`
}

// 抢答记录（quiz+question+user 唯一，防重复抢答/得分）
type RushRecord struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	QuizID     int64     `gorm:"notNull;uniqueIndex:idx_rush" json:"quiz_id"`
	QuestionID int64     `gorm:"notNull;uniqueIndex:idx_rush" json:"question_id"`
	UserID     int64     `gorm:"notNull;uniqueIndex:idx_rush" json:"user_id"`
	ServerTime int64     `gorm:"notNull" json:"server_time"` // 服务器纳秒时间戳
	Rank       int       `gorm:"notNull" json:"rank"`
	Score      int       `json:"score"`
	CreatedAt  time.Time `json:"created_at"`
}

// AutoMigrate 全部表
func AllModels() []any {
	return []any{
		&User{},
		&Quiz{},
		&Question{},
		&QuestionOption{},
		&Participant{},
		&Answer{},
		&RushRecord{},
	}
}
