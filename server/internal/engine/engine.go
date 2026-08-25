package engine

import (
	"sync"

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
	tickRush     *ticker
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
	e.runtimes[quizID] = rt
	return rt, nil
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
