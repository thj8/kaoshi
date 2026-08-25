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

func (h *ControlHandler) Start(c *gin.Context)    { h.wrap(c, func() error { return h.Eng.Start(id(c)) }) }
func (h *ControlHandler) Pause(c *gin.Context)    { h.wrap(c, func() error { return h.Eng.Pause(id(c)) }) }
func (h *ControlHandler) Resume(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.Resume(id(c)) }) }
func (h *ControlHandler) Next(c *gin.Context)     { h.wrap(c, func() error { return h.Eng.Next(id(c)) }) }
func (h *ControlHandler) Previous(c *gin.Context) { h.wrap(c, func() error { return h.Eng.Previous(id(c)) }) }
func (h *ControlHandler) Reveal(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.Reveal(id(c)) }) }
func (h *ControlHandler) End(c *gin.Context) {
	h.wrap(c, func() error { return h.Eng.End(id(c)) })
}
func (h *ControlHandler) RushStart(c *gin.Context) { h.wrap(c, func() error { return h.Eng.RushStart(id(c)) }) }
func (h *ControlHandler) RushEnd(c *gin.Context)   { h.wrap(c, func() error { return h.Eng.RushEnd(id(c)) }) }

// Statistics GET /api/admin/quiz/:id/statistics 实时+最终统计
func (h *ControlHandler) Statistics(c *gin.Context) {
	quizID := id(c)
	var quiz model.Quiz
	if err := h.DB.First(&quiz, quizID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "msg": "答题不存在"})
		return
	}
	stats := h.Eng.Statistics(quizID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": stats})
}

func id(c *gin.Context) int64 {
	n, _ := parseInt64(c.Param("id"))
	return n
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
