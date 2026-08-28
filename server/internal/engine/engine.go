package engine

import (
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"kaoshi/internal/model"
	"kaoshi/internal/ws"
)

// Engine 答题引擎：每场活动一个 Runtime，懒加载
type Engine struct {
	DB  *gorm.DB
	RDB *redis.Client
	Hub *ws.Hub

	mu       sync.Mutex
	runtimes map[int64]*Runtime
}

// Runtime 单场答题的内存运行时（可从 DB 重建）
type Runtime struct {
	mu        sync.Mutex
	engine    *Engine
	quiz      *model.Quiz
	questions []model.Question // 按 sort 排序
	options   map[int64][]model.QuestionOption

	curIndex int        // 当前题下标（0-based，-1=未发布）
	deadline int64      // 当前题截止毫秒时间戳（0=无）
	timer    *SyncTimer // 倒计时器
	tick     *ticker    // 每秒广播
	pausedRemain int64  // 暂停时剩余毫秒

	// 抢答窗口
	rushDeadline int64
	rushOpenAt   int64 // 窗口开启毫秒时间戳（0=已开启/无窗口）；开启前 RushSubmit 一律拒绝
	rushTimer   *SyncTimer
	tickRush    *ticker
}

func New(db *gorm.DB, rdb *redis.Client, hub *ws.Hub) *Engine {
	return &Engine{DB: db, RDB: rdb, Hub: hub, runtimes: map[int64]*Runtime{}}
}

// Get 懒加载运行时（含状态重建）
func (e *Engine) Get(quizID int64) (*Runtime, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	rt, ok := e.runtimes[quizID]
	if ok {
		return rt, nil
	}
	var quiz model.Quiz
	if err := e.DB.First(&quiz, quizID).Error; err != nil {
		return nil, err
	}
	var questions []model.Question
	if err := e.DB.Where("quiz_id = ?", quizID).Order("sort ASC").Find(&questions).Error; err != nil {
		return nil, err
	}
	rt = &Runtime{
		engine:    e,
		quiz:      &quiz,
		questions: questions,
		options:   map[int64][]model.QuestionOption{},
		curIndex:  -1,
	}
	e.recoverRuntime(rt)
	e.runtimes[quizID] = rt
	return rt, nil
}

// recoverRuntime 服务重启后从 DB 恢复进行中的状态（不恢复倒计时：
// 重启视为异常，题目停在当前题，由管理员手动 next/previous/reveal 继续，避免重启瞬间全体强制收卷）
func (e *Engine) recoverRuntime(rt *Runtime) {
	q := rt.quiz
	// 考试模式：按 started_at + total_time 重新武装全局倒计时（到时自动收卷）
	if q.Mode == model.ModeExam && q.Status == model.QuizStatusRunning && q.StartedAt != nil && q.TotalTime > 0 {
		endAt := q.StartedAt.Add(time.Duration(q.TotalTime) * time.Second)
		remain := time.Until(endAt)
		if remain <= 0 {
			// 已到时：异步收卷（此处处于 Get 的引擎锁内，不能同步调 End→Get）
			go e.End(q.ID)
		} else {
			rt.deadline = endAt.UnixMilli()
			rt.startTimer(remain, func() { e.End(q.ID) })
			rt.startTickerLocked(0)
		}
		log.Printf("[engine] quiz %d 考试恢复: 剩余 %s", q.ID, remain)
		return
	}
	switch q.Status {
	case model.QuizStatusAnswering, model.QuizStatusRevealing, model.QuizStatusRushing:
		// 找到最近一道已有作答记录的题作为恢复位置；若无则从第 1 题重发
		if len(rt.questions) == 0 {
			return
		}
		idx := 0
		for i := len(rt.questions) - 1; i >= 0; i-- {
			var cnt int64
			e.DB.Model(&model.Answer{}).
				Where("quiz_id = ? AND question_id = ?", q.ID, rt.questions[i].ID).
				Count(&cnt)
			if cnt > 0 {
				idx = i
				break
			}
		}
		rt.curIndex = idx
		log.Printf("[engine] quiz %d 状态恢复: status=%s curIndex=%d（重启前进行中）", q.ID, q.Status, idx)
	}
}

// GetOptions 题目选项（缓存，外部调用，自带锁）
func (rt *Runtime) GetOptions(e *Engine, questionID int64) []model.QuestionOption {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.getOptionsLocked(e, questionID)
}

// getOptionsLocked 内部版本：调用方必须已持有 rt.mu
func (rt *Runtime) getOptionsLocked(e *Engine, questionID int64) []model.QuestionOption {
	if opts, ok := rt.options[questionID]; ok {
		return opts
	}
	var opts []model.QuestionOption
	e.DB.Where("question_id = ?", questionID).Order("sort ASC").Find(&opts)
	rt.options[questionID] = opts
	return opts
}

// Quiz 只读访问
func (rt *Runtime) Quiz() *model.Quiz { return rt.quiz }
