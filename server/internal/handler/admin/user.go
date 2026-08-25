package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"kaoshi/internal/model"
)

// ---------- 用户管理 ----------

// UserRow 用户列表行（含聚合统计）
type UserRow struct {
	ID         int64      `json:"id"`
	Username   string     `json:"username"`
	Nickname   string     `json:"nickname"`
	CreatedAt  time.Time  `json:"created_at"`
	QuizCount  int64      `json:"quiz_count"`   // 参加场次数
	TotalScore int64      `json:"total_score"`  // 总得分
	CorrectCnt int64      `json:"correct_cnt"`  // 总答对
	WrongCnt   int64      `json:"wrong_cnt"`    // 总答错
	AnswerCnt  int64      `json:"answer_cnt"`   // 总答题数
	LastJoined *time.Time `json:"last_joined"`  // 最近参与时间
}

// ListUsers GET /api/admin/users?keyword=
func (h *Handler) ListUsers(c *gin.Context) {
	keyword := "%" + c.Query("keyword") + "%"

	base := h.DB.Table("users").
		Select(`users.id, users.username, users.nickname, users.created_at,
			COUNT(participants.id) AS quiz_count,
			COALESCE(SUM(participants.score),0) AS total_score,
			COALESCE(SUM(participants.correct_count),0) AS correct_cnt,
			COALESCE(SUM(participants.wrong_count),0) AS wrong_cnt,
			MAX(participants.joined_at) AS last_joined`).
		Joins("LEFT JOIN participants ON participants.user_id = users.id").
		Group("users.id, users.username, users.nickname, users.created_at")

	var rows []struct {
		model.User
		QuizCount  int64      `json:"quiz_count"`
		TotalScore int64      `json:"total_score"`
		CorrectCnt int64      `json:"correct_cnt"`
		WrongCnt   int64      `json:"wrong_cnt"`
		LastJoined *time.Time `json:"last_joined"`
	}
	q := base
	if kw := c.Query("keyword"); kw != "" {
		q = q.Where("users.nickname LIKE ? OR users.username LIKE ?", keyword, keyword)
	}
	q.Order("users.id DESC").Limit(500).Scan(&rows)

	// 总答题数单独聚合（answers 表）
	ansCnt := map[int64]int64{}
	type ar struct {
		UserID int64
		Cnt    int64
	}
	var ars []ar
	h.DB.Table("answers").Select("user_id, COUNT(*) as cnt").Group("user_id").Scan(&ars)
	for _, r := range ars {
		ansCnt[r.UserID] = r.Cnt
	}

	out := make([]UserRow, len(rows))
	for i, r := range rows {
		out[i] = UserRow{
			ID: r.ID, Username: r.Username, Nickname: r.Nickname,
			CreatedAt:  r.CreatedAt,
			QuizCount:  r.QuizCount,
			TotalScore: r.TotalScore,
			CorrectCnt: r.CorrectCnt,
			WrongCnt:   r.WrongCnt,
			AnswerCnt:  ansCnt[r.ID],
			LastJoined: r.LastJoined,
		}
	}
	ok(c, out)
}

// CreateUser POST /api/admin/users（管理员新增用户）
func (h *Handler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2,max=32"`
		Password string `json:"password" binding:"required,min=4,max=64"`
		Nickname string `json:"nickname" binding:"required,min=1,max=32"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "用户名2-32位、密码至少4位、昵称1-32位")
		return
	}
	var cnt int64
	h.DB.Model(&model.User{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		fail(c, 400, "用户名已存在")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, 500, "系统错误")
		return
	}
	user := model.User{Username: req.Username, PasswordHash: string(hash), Nickname: req.Nickname}
	if err := h.DB.Create(&user).Error; err != nil {
		fail(c, 500, "创建失败")
		return
	}
	ok(c, gin.H{"id": user.ID, "username": user.Username, "nickname": user.Nickname})
}

// UpdateUser PUT /api/admin/users/:id （改昵称/重置密码，均可选）
func (h *Handler) UpdateUser(c *gin.Context) {
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	var req struct {
		Nickname string `json:"nickname" binding:"omitempty,min=1,max=32"`
		Password string `json:"password" binding:"omitempty,min=4,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "参数不合法")
		return
	}
	if req.Nickname == "" && req.Password == "" {
		fail(c, 400, "请填写新昵称或新密码")
		return
	}
	if req.Nickname != "" {
		var cnt int64
		h.DB.Model(&model.User{}).Where("nickname = ? AND id != ?", req.Nickname, user.ID).Count(&cnt)
		if cnt > 0 {
			fail(c, 400, "该昵称已被使用")
			return
		}
		user.Nickname = req.Nickname
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(c, 500, "系统错误")
			return
		}
		user.PasswordHash = string(hash)
	}
	h.DB.Save(&user)
	ok(c, gin.H{"id": user.ID, "username": user.Username, "nickname": user.Nickname})
}

// DeleteUser DELETE /api/admin/users/:id （级联删除参与/答题/抢答记录）
func (h *Handler) DeleteUser(c *gin.Context) {
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.RushRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.Answer{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.Participant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	})
	if err != nil {
		fail(c, 500, "删除失败："+err.Error())
		return
	}
	ok(c, nil)
}

// UserDetail GET /api/admin/users/:id （参与明细）
func (h *Handler) UserDetail(c *gin.Context) {
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		fail(c, 404, "用户不存在")
		return
	}
	type partRow struct {
		QuizID       int64  `json:"quiz_id"`
		Title        string `json:"title"`
		Status       string `json:"status"`
		Score        int    `json:"score"`
		CorrectCount int    `json:"correct_count"`
		WrongCount   int    `json:"wrong_count"`
		Rank         int    `json:"rank"`
		JoinedAt     string `json:"joined_at"`
	}
	var parts []partRow
	h.DB.Table("participants").
		Select(`participants.quiz_id, quizzes.title, quizzes.status,
			participants.score, participants.correct_count, participants.wrong_count, participants.joined_at`).
		Joins("JOIN quizzes ON quizzes.id = participants.quiz_id").
		Where("participants.user_id = ?", user.ID).
		Order("participants.joined_at DESC").Scan(&parts)

	// 每场的排名
	for i := range parts {
		var rank int64
		h.DB.Table("participants").
			Where("quiz_id = ? AND (score > ? OR (score = ? AND correct_count > ?))",
				parts[i].QuizID, parts[i].Score, parts[i].Score, parts[i].CorrectCount).
			Count(&rank)
		parts[i].Rank = int(rank) + 1
	}
	ok(c, gin.H{"user": user, "parts": parts})
}
