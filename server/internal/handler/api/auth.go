package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"kaoshi/internal/auth"
	"kaoshi/internal/middleware"
	"kaoshi/internal/model"
)

// userLoginLimiter 用户登录失败限速：同 IP 10 次失败锁 1 分钟（比 admin 宽松，兼顾多人同一出口）
var userLoginLimiter = middleware.NewIPLimiter(10, time.Minute)

// ---------- 用户登录 ----------

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/auth/login
func (h *Handler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if !userLoginLimiter.Allow(ip) {
		fail(c, 429, "尝试过于频繁，请 1 分钟后再试")
		return
	}
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "请输入用户名和密码")
		return
	}
	var user model.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		userLoginLimiter.Fail(ip)
		fail(c, 401, "用户名或密码错误")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		userLoginLimiter.Fail(ip)
		fail(c, 401, "用户名或密码错误")
		return
	}
	ok(c, gin.H{"token": h.signGlobalToken(&user), "user": userBrief(&user)})
}

// Me GET /api/auth/me
func (h *Handler) Me(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var user model.User
	if err := h.DB.First(&user, claims.UserID).Error; err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	ok(c, userBrief(&user))
}

// ---------- 加入答题（需登录） ----------

type joinReq struct {
	QuizCode string `json:"quiz_id" binding:"required"` // 值为 10 位随机码
}

// Join POST /api/join：已登录用户加入指定答题，返回答题作用域 token
func (h *Handler) Join(c *gin.Context) {
	claims := c.MustGet("claims").(*auth.Claims)
	var req joinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "缺少答题编号")
		return
	}
	var quiz model.Quiz
	if err := h.DB.Where("code = ?", req.QuizCode).First(&quiz).Error; err != nil {
		fail(c, 404, "答题不存在，请检查链接")
		return
	}
	// 邀请制：名单非空 = 仅名单内用户可加入
	var invCnt int64
	h.DB.Model(&model.QuizInvitee{}).Where("quiz_id = ?", quiz.ID).Count(&invCnt)
	if invCnt > 0 {
		var me int64
		h.DB.Model(&model.QuizInvitee{}).Where("quiz_id = ? AND user_id = ?", quiz.ID, claims.UserID).Count(&me)
		if me == 0 {
			fail(c, 403, "该比赛未对你开放")
			return
		}
	}

	// 幂等加入；已结束时仅老参与者可重新领 token 回看成绩
	var p model.Participant
	if err := h.DB.Where("quiz_id = ? AND user_id = ?", quiz.ID, claims.UserID).First(&p).Error; err != nil {
		if quiz.Status == model.QuizStatusFinished {
			fail(c, 400, "答题已结束")
			return
		}
		p = model.Participant{QuizID: quiz.ID, UserID: claims.UserID, JoinedAt: time.Now()}
		h.DB.Create(&p)
	}

	// 答题作用域 token（含 quiz_id，供 WS 与答题接口鉴权）
	token, err := auth.Sign(&auth.Claims{
		Role: auth.RoleUser, UserID: claims.UserID,
		Nick: claims.Nick, QuizID: quiz.ID,
	})
	if err != nil {
		fail(c, 500, "生成令牌失败")
		return
	}
	ok(c, gin.H{
		"token": token,
		"quiz":  quizBriefOf(&quiz),
		"user":  gin.H{"id": claims.UserID, "nickname": claims.Nick},
	})
}

func (h *Handler) signGlobalToken(u *model.User) string {
	token, _ := auth.Sign(&auth.Claims{
		Role: auth.RoleUser, UserID: u.ID, Nick: u.Nickname,
	})
	return token
}

func userBrief(u *model.User) gin.H {
	return gin.H{"id": u.ID, "username": u.Username, "nickname": u.Nickname}
}
