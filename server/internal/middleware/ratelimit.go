package middleware

import (
	"sync"
	"time"
)

// IPLimiter 同 IP 登录失败限速：失败达到 maxFails 次锁定 window 时长
// ponytail: 单机内存版够用；多实例部署换 Redis
type IPLimiter struct {
	mu       sync.Mutex
	maxFails int
	window   time.Duration
	fails    map[string]*iplimiterState
}

type iplimiterState struct {
	count int
	until time.Time
}

func NewIPLimiter(maxFails int, window time.Duration) *IPLimiter {
	return &IPLimiter{maxFails: maxFails, window: window, fails: map[string]*iplimiterState{}}
}

func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	fs := l.fails[ip]
	return fs == nil || time.Now().After(fs.until)
}

func (l *IPLimiter) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fs := l.fails[ip]
	if fs == nil {
		fs = &iplimiterState{}
		l.fails[ip] = fs
	}
	fs.count++
	if fs.count >= l.maxFails {
		fs.until = time.Now().Add(l.window)
		fs.count = 0
	}
}
