package engine

import "time"

func nowMilli() int64 { return time.Now().UnixMilli() }

// clip 按字符（非字节）截断，中文安全；服务端日志用
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
