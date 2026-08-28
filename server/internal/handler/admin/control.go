package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kaoshi/internal/engine"
	"kaoshi/internal/model"
)

// ControlHandler 答题流程控制（依赖 engine）
type ControlHandler struct {
	DB  *gorm.DB
	Eng *engine.Engine
}

func NewControl(db *gorm.DB, eng *engine.Engine) *ControlHandler {
	return &ControlHandler{DB: db, Eng: eng}
}

func (h *ControlHandler) wrap(c *gin.Context, fn func() error) {
	if err := fn(); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": nil})
}

func (h *ControlHandler) Start(c *gin.Context)    { h.wrap(c, func() error { return h.Eng.Start(h.id(c)) }) }
func (h *ControlHandler) Pause(c *gin.Context)    { h.wrap(c, func() error { return h.Eng.Pause(h.id(c)) }) }
func (h *ControlHandler) Resume(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.Resume(h.id(c)) }) }
func (h *ControlHandler) Next(c *gin.Context)     { h.wrap(c, func() error { return h.Eng.Next(h.id(c)) }) }
func (h *ControlHandler) Previous(c *gin.Context) { h.wrap(c, func() error { return h.Eng.Previous(h.id(c)) }) }
func (h *ControlHandler) Reveal(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.Reveal(h.id(c)) }) }
func (h *ControlHandler) End(c *gin.Context) {
	h.wrap(c, func() error { return h.Eng.End(h.id(c)) })
}
func (h *ControlHandler) RushStart(c *gin.Context) { h.wrap(c, func() error { return h.Eng.RushStart(h.id(c)) }) }
func (h *ControlHandler) RushEnd(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.RushEnd(h.id(c)) }) }
func (h *ControlHandler) Reset(c *gin.Context)    { h.wrap(c, func() error { return h.Eng.Reset(h.id(c)) }) }

// Statistics GET /api/admin/quiz/:id/statistics 实时+最终统计
func (h *ControlHandler) Statistics(c *gin.Context) {
	quizID := h.id(c)
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "答题不存在"})
		return
	}
	stats := h.Eng.Statistics(quizID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": stats})
}

// id 路径参数为 10 位随机码，解析为内部 quizID（不存在返回 0）
func (h *ControlHandler) id(c *gin.Context) int64 {
	var q model.Quiz
	if h.DB.Where("code = ?", c.Param("id")).First(&q).Error != nil {
		return 0
	}
	return q.ID
}

func parseInt64(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	n := int64(0)
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int64(ch-'0')
	}
	return n, true
}
