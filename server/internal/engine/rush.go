package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"kaoshi/internal/model"
	"kaoshi/internal/ws"
)

// Redis key 设计：
//   rush:{quizID}:{questionID}        ZSET  member=userID score=服务器纳秒（成功者，按时间排序）
//   rush:{quizID}:{questionID}:fail   SET   抢答失败者（名额已满被拒）
//
// Lua 脚本保证：判序 + 名额限制 + 防重复 全部原子完成，
// 100 并发下第一名唯一、结果稳定（score 为服务器收到时间，非客户端时间）

var rushScript = redis.NewScript(`
local score = redis.call('ZSCORE', KEYS[1], ARGV[1])
if score then return -1 end
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then return -3 end
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[2]) then
	redis.call('SADD', KEYS[2], ARGV[1])
	return -2
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
redis.call('EXPIRE', KEYS[1], 3600)
redis.call('EXPIRE', KEYS[2], 3600)
return redis.call('ZRANK', KEYS[1], ARGV[1])
`)

func rushKeys(quizID, questionID int64) (string, string) {
	base := fmt.Sprintf("rush:%d:%d", quizID, questionID)
	return base, base + ":fail"
}

var (
	ErrRushDisabled  = errors.New("该答题未开启抢答")
	ErrRushNotActive = errors.New("当前不在抢答阶段")
	ErrAlreadyRushed = errors.New("已抢答过")
	ErrRushFull      = errors.New("很遗憾，本题抢答资格已被其他用户获得")
)

// ---------- 管理端：开始抢答 ----------

// RushStart 开始抢答：当前题进入抢答窗口
func (e *Engine) RushStart(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if !rt.quiz.RushEnabled {
		return ErrRushDisabled
	}
	if rt.quiz.Status == model.QuizStatusRushing {
		return nil // 幂等
	}
	if rt.quiz.Status != model.QuizStatusAnswering {
		return errors.New("请先发布题目再开始抢答")
	}
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) {
		return errors.New("没有当前题目")
	}
	q := rt.questions[rt.curIndex]

	// 本题已完成过抢答（有成功记录）则不允许重复抢
	var cnt int64
	e.DB.Model(&model.RushRecord{}).
		Where("quiz_id = ? AND question_id = ?", quizID, q.ID).Count(&cnt)
	if cnt > 0 {
		return errors.New("本题已完成抢答，请公布答案或下一题")
	}

	// 冻结原题倒计时，清理旧窗口数据
	rt.stopTimer()
	rt.stopTicker()
	rt.deadline = 0

	k1, k2 := rushKeys(quizID, q.ID)
	e.RDB.Del(context.Background(), k1, k2)

	rt.rushDeadline = time.Now().Add(time.Duration(rt.quiz.RushTime) * time.Second).UnixMilli()
	rt.quiz.Status = model.QuizStatusRushing
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusRushing)

	e.Hub.Broadcast(quizID, ws.EventRushStart, &ws.RushStartData{
		QuestionID: q.ID,
		Winners:    rt.quiz.RushWinnerCount,
		DeadlineAt: rt.rushDeadline,
		ServerNow:  nowMilli(),
	})

	// 抢答窗口计时 + 每秒倒计时广播
	qID := q.ID
	rt.startRushTimerLocked(time.Duration(rt.quiz.RushTime)*time.Second, func() {
		e.RushEnd(quizID)
	})
	rt.startRushTickerLocked(qID)
	return nil
}

// RushEnd 结束抢答（窗口到时 / 名额满 / 管理员手动）
func (e *Engine) RushEnd(quizID int64) error {
	rt, err := e.Get(quizID)
	if err != nil {
		return ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return e.rushEndLocked(rt)
}

// rushEndLocked 结束抢答窗口（调用方持锁）
func (e *Engine) rushEndLocked(rt *Runtime) error {
	quizID := rt.quiz.ID
	if rt.quiz.Status != model.QuizStatusRushing {
		return ErrRushNotActive
	}
	rt.stopRushTimer()
	rt.stopRushTicker()
	rt.rushDeadline = 0

	q := rt.questions[rt.curIndex]
	winners := e.redisWinners(rt, q.ID)

	var answerDeadline int64
	if len(winners) > 0 {
		// 获答者进入答题：状态 ANSWERING + 专属倒计时
		rt.quiz.Status = model.QuizStatusAnswering
		d := time.Duration(rt.quiz.RushAnswerTime) * time.Second
		answerDeadline = time.Now().Add(d).UnixMilli()
		rt.deadline = answerDeadline
		rt.startTimer(d, func() { e.forceCollect(quizID, q.ID) })
		rt.startTickerLocked(q.ID)
	} else {
		// 无人抢答成功：回到 ANSWERING（无倒计时），管理员可重抢/公布/下一题
		rt.quiz.Status = model.QuizStatusAnswering
		rt.deadline = 0
	}
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusAnswering)

	e.Hub.Broadcast(quizID, ws.EventRushEnd, &ws.RushEndData{
		QuestionID:       q.ID,
		Winners:          winners,
		AnswerDeadlineAt: answerDeadline,
		ServerNow:        nowMilli(),
	})
		if rt.quiz.ShowRanking {
			e.Hub.Broadcast(quizID, ws.EventRankingUpdate, &ws.RankingData{Items: e.buildRanking(quizID, 50)})
		}
		e.broadcastStatistics(quizID, q.ID)
		return nil
	}

// ---------- 用户端：抢答提交 ----------

// RushSubmit 抢答：Redis Lua 原子判序，成功即记 DB + 加奖励分
func (e *Engine) RushSubmit(quizID, questionID, userID int64) (*ws.RushResultData, error) {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.quiz.Status != model.QuizStatusRushing {
		// 窗口刚关闭（名额已满自动结束）：给出准确失败原因
		k1, k2 := rushKeys(quizID, questionID)

		if e.RDB.Exists(context.Background(), k1).Val() > 0 || e.RDB.Exists(context.Background(), k2).Val() > 0 {
			return nil, ErrRushFull
		}
		return nil, ErrRushNotActive
	}
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) || rt.questions[rt.curIndex].ID != questionID {
		return nil, errors.New("当前题目不匹配")
	}
	// 窗口校验（宽限 500ms 网络延迟），以服务器时间为准
	if rt.rushDeadline > 0 && nowMilli() > rt.rushDeadline+500 {
		return nil, errors.New("抢答时间已到")
	}

	k1, k2 := rushKeys(quizID, questionID)
	res, err := rushScript.Run(context.Background(), e.RDB, []string{k1, k2},
		fmt.Sprintf("%d", userID), rt.quiz.RushWinnerCount, time.Now().UnixNano()).Int64()
	if err != nil {
		return nil, errors.New("抢答服务繁忙，请重试")
	}

	switch res {
	case -1:
		return nil, ErrAlreadyRushed
	case -2, -3:
		// 名额已满 → 失败（fail set 已记录，重连恢复状态用）
		e.Hub.SendToUser(quizID, userID, ws.EventRushFailed, &ws.RushResultData{
			QuestionID: questionID, Rank: 0, Reason: "full",
		})
		return nil, ErrRushFull
	}

	rank := int(res) + 1 // 0-based → 1-based
	bonus := rt.quiz.RushBonusScore

	// DB 记录（唯一索引兜底防重复计分）
	rec := model.RushRecord{
		QuizID: quizID, QuestionID: questionID, UserID: userID,
		ServerTime: time.Now().UnixNano(), Rank: rank, Score: bonus,
	}
	if err := e.DB.Create(&rec).Error; err != nil {
		// 并发双保险触发：从 ZSET 移除，回滚本次
		e.RDB.ZRem(context.Background(), k1, fmt.Sprintf("%d", userID))
		return nil, ErrAlreadyRushed
	}

	// 奖励分立即入账
	e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).
		Update("score", gorm.Expr("score + ?", bonus))
	var p model.Participant
	e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p)

	var u model.User
	e.DB.First(&u, userID)

	result := &ws.RushResultData{
		QuestionID: questionID, Rank: rank, Nickname: u.Nickname,
		Bonus: bonus, Score: p.Score,
	}
	e.Hub.SendToUser(quizID, userID, ws.EventRushSuccess, result)
	e.broadcastStatistics(quizID, questionID)

	// 名额满 → 提前结束抢答窗口（持锁内直接调用 rushEndLocked）
	if rank >= rt.quiz.RushWinnerCount {
		_ = e.rushEndLocked(rt)
	}
	return result, nil
}

// myRushRank 某用户在当前窗口的抢答状态：>0 成功名次，-1 失败，0 未抢
func (e *Engine) myRushRank(quizID, questionID, userID int64) int {
	k1, k2 := rushKeys(quizID, questionID)
	ctx := context.Background()
	uid := fmt.Sprintf("%d", userID)
	if e.RDB.SIsMember(ctx, k2, uid).Val() {
		return -1
	}
	rank, err := e.RDB.ZRank(ctx, k1, uid).Result()
	if err == nil {
		return int(rank) + 1
	}
	return 0
}

// rushWinnersFromDB 从 DB 读取本题抢答成功者（窗口已结束场景）
func (e *Engine) rushWinnersFromDB(quizID, questionID int64) []ws.RushWinner {
	var recs []model.RushRecord
	e.DB.Where("quiz_id = ? AND question_id = ?", quizID, questionID).Order("`rank` ASC").Find(&recs)
	if len(recs) == 0 {
		return nil
	}
	uids := make([]int64, len(recs))
	for i, r := range recs {
		uids[i] = r.UserID
	}
	var users []model.User
	e.DB.Where("id IN ?", uids).Find(&users)
	nick := map[int64]string{}
	for _, u := range users {
		nick[u.ID] = u.Nickname
	}
	out := make([]ws.RushWinner, len(recs))
	for i, r := range recs {
		out[i] = ws.RushWinner{UserID: r.UserID, Nickname: nick[r.UserID], Rank: r.Rank, Bonus: r.Score}
	}
	return out
}

// redisWinners 读取当前抢答成功者（含昵称/奖励分）
func (e *Engine) redisWinners(rt *Runtime, questionID int64) []ws.RushWinner {
	k1, _ := rushKeys(rt.quiz.ID, questionID)
	ctx := context.Background()
	members, err := e.RDB.ZRange(ctx, k1, 0, -1).Result() // 按 score 升序（抢答先后）
	if err != nil || len(members) == 0 {
		return nil
	}
	out := make([]ws.RushWinner, 0, len(members))
	for i, m := range members {
		var uid int64
		fmt.Sscanf(m, "%d", &uid)
		var u model.User
		if e.DB.First(&u, uid).Error != nil {
			continue
		}
		out = append(out, ws.RushWinner{
			UserID: uid, Nickname: u.Nickname, Rank: i + 1, Bonus: rt.quiz.RushBonusScore,
		})
	}
	return out
}

// isRushQuestion 本题是否已发生过抢答（存在成功记录）
func (e *Engine) isRushQuestion(quizID, questionID int64) bool {
	var cnt int64
	e.DB.Model(&model.RushRecord{}).
		Where("quiz_id = ? AND question_id = ?", quizID, questionID).Count(&cnt)
	return cnt > 0
}

// rushWinnerIDs 本题抢答成功者 userID 集合
func (e *Engine) rushWinnerIDs(quizID, questionID int64) []int64 {
	var ids []int64
	e.DB.Model(&model.RushRecord{}).
		Where("quiz_id = ? AND question_id = ?", quizID, questionID).
		Order("`rank` ASC").Pluck("user_id", &ids)
	return ids
}

// ---------- Runtime 抢答计时器（持锁调用） ----------

func (rt *Runtime) startRushTimerLocked(d time.Duration, fn func()) {
	rt.stopRushTimer()
	rt.rushTimer = NewSyncTimer(d, fn)
}

func (rt *Runtime) stopRushTimer() {
	if rt.rushTimer != nil {
		rt.rushTimer.Cancel()
		rt.rushTimer = nil
	}
}

func (rt *Runtime) startRushTickerLocked(questionID int64) {
	rt.stopRushTicker()
	e := rt.engine
	if e == nil {
		return
	}
	qid := rt.quiz.ID
	rt.tickRush = startTicker(func() {
		rt.mu.Lock()
		dl := rt.rushDeadline
		rt.mu.Unlock()
		if dl <= 0 {
			return
		}
		remain := (dl - nowMilli()) / 1000
		if remain < 0 {
			remain = 0
		}
		e.Hub.Broadcast(qid, ws.EventQuestionCountd, &ws.CountdownData{
			QuestionID: questionID, RemainSec: int(remain), DeadlineAt: dl,
		})
	})
}
