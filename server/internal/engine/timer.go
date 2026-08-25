package engine

import (
	"sync"
	"time"
)

// SyncTimer 可暂停的倒计时器（到点回调 forceFn）
type SyncTimer struct {
	mu       sync.Mutex
	duration time.Duration
	deadline time.Time
	timer    *time.Timer
	fired    bool
}

func NewSyncTimer(d time.Duration, onFire func()) *SyncTimer {
	t := &SyncTimer{duration: d, fired: false}
	t.timer = time.AfterFunc(d, func() {
		t.mu.Lock()
		fired := t.fired
		t.mu.Unlock()
		if !fired {
			onFire()
		}
	})
	t.deadline = time.Now().Add(d)
	return t
}

// Remain 剩余毫秒
func (t *SyncTimer) Remain() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	r := time.Until(t.deadline).Milliseconds()
	if r < 0 {
		return 0
	}
	return r
}

// Deadline 截止毫秒时间戳
func (t *SyncTimer) Deadline() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.deadline.UnixMilli()
}

// Pause 暂停，返回剩余时长
func (t *SyncTimer) Pause() time.Duration {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timer.Stop()
	remain := time.Until(t.deadline)
	if remain < 0 {
		remain = 0
	}
	return remain
}

// Cancel 取消
func (t *SyncTimer) Cancel() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timer.Stop()
	t.fired = true
}

// tick 倒计时广播（由 Runtime 调度）
type ticker struct {
	mu     sync.Mutex
	stop   chan struct{}
	stopped bool
}

func startTicker(fn func()) *ticker {
	tk := &ticker{stop: make(chan struct{})}
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				fn()
			case <-tk.stop:
				return
			}
		}
	}()
	return tk
}

func (tk *ticker) Stop() {
	tk.mu.Lock()
	defer tk.mu.Unlock()
	if !tk.stopped {
		tk.stopped = true
		close(tk.stop)
	}
}
