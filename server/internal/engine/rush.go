package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

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
	ErrRushRepeatWin  = errors.New("重复抢答：你已抢到过本题")
	ErrRushRepeatFail = errors.New("重复抢答：此前未抢到，结果不变")
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
	return e.rushStartLocked(rt)
}

// rushStartLocked 进入抢答窗口（调用方持有 rt.mu）；发布非必答题时自动调用
func (e *Engine) rushStartLocked(rt *Runtime) error {
	quizID := rt.quiz.ID
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) {
		return errors.New("没有当前题目")
	}
	q := rt.questions[rt.curIndex]

	// 必答题（非抢答模式）不允许抢答：抢答与否只由题目设置决定
	if q.Required && rt.quiz.Mode != model.ModeRush {
		return errors.New("必答题不能抢答")
	}

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

	// 开抢倒计时：窗口在 open_at 后才真正开启；截止 = 开启 + 窗口时长（时长不因倒计时缩短）
	rushCountdown := rt.quiz.RushCountdown
	if rushCountdown < 0 {
		rushCountdown = 0
	}
	rt.rushOpenAt = nowMilli() + int64(rushCountdown)*1000
	rt.rushDeadline = rt.rushOpenAt + int64(rt.quiz.RushTime)*1000
	rt.quiz.Status = model.QuizStatusRushing
	e.DB.Model(&model.Quiz{}).Where("id = ?", quizID).Update("status", model.QuizStatusRushing)

	e.Hub.Broadcast(quizID, ws.EventRushStart, &ws.RushStartData{
		QuestionID: q.ID,
		Winners:    rt.quiz.RushWinnerCount,
		OpenAt:     rt.rushOpenAt,
		DeadlineAt: rt.rushDeadline,
		ServerNow:  nowMilli(),
	})

	// 抢答窗口计时 + 每秒倒计时广播
	qID := q.ID
	rt.startRushTimerLocked(time.Duration(rt.rushDeadline-nowMilli())*time.Millisecond, func() {
		e.RushEnd(quizID)
	})
	rt.startRushTickerLocked(qID)
	log.Printf("[rush] quiz=%d 第%d题 开启抢答（名额%d 窗口%ds）", quizID, rt.curIndex+1, rt.quiz.RushWinnerCount, rt.quiz.RushTime)
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
	rt.rushOpenAt = 0

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
func (e *Engine) RushSubmit(quizID, questionID, userID int64) (result *ws.RushResultData, err error) {
	rt, err := e.Get(quizID)
	if err != nil {
		return nil, ErrNotFound
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// 结果日志（defer 覆盖所有分支；发起日志在 handler，接口调用即记）
	var u model.User
	e.DB.First(&u, userID)
	idx := rt.curIndex + 1
	rank := 0
	defer func() {
		if err == nil {
			log.Printf("[rush] quiz=%d 第%d题 %s 抢到 rank=%d/%d", quizID, idx, u.Nickname, rank, rt.quiz.RushWinnerCount)
		} else {
			log.Printf("[rush] quiz=%d 第%d题 %s 未抢到：%v", quizID, idx, u.Nickname, err)
		}
	}()

	if rt.quiz.Status != model.QuizStatusRushing {
		// 窗口已关（名额满自动结束）：先区分本人重复抢答，再报满员/未开抢
		k1, k2 := rushKeys(quizID, questionID)
		uid := fmt.Sprintf("%d", userID)
		ctx := context.Background()
		if e.RDB.ZScore(ctx, k1, uid).Err() == nil {
			return nil, ErrRushRepeatWin
		}
		if e.RDB.SIsMember(ctx, k2, uid).Val() {
			return nil, ErrRushRepeatFail
		}
		if e.RDB.Exists(ctx, k1).Val() > 0 || e.RDB.Exists(ctx, k2).Val() > 0 {
			return nil, ErrRushFull
		}
		return nil, ErrRushNotActive
	}
	if rt.curIndex < 0 || rt.curIndex >= len(rt.questions) || rt.questions[rt.curIndex].ID != questionID {
		return nil, errors.New("当前题目不匹配")
	}
	var pc int64
	e.DB.Model(&model.Participant{}).Where("quiz_id = ? AND user_id = ?", quizID, userID).Count(&pc)
	if pc == 0 {
		return nil, errors.New("参赛信息不存在，请重新加入本场答题")
	}
	// 开抢倒计时未结束：拒绝（防 API 直连绕过前端倒计时抢跑）
	if rt.rushOpenAt > 0 && nowMilli() < rt.rushOpenAt {
		return nil, errors.New("抢答尚未开始，请稍候")
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
		return nil, ErrRushRepeatWin
	case -3:
		return nil, ErrRushRepeatFail
	case -2:
		// 名额已满 → 失败（fail set 已记录，重连恢复状态用）
		e.Hub.SendToUser(quizID, userID, ws.EventRushFailed, &ws.RushResultData{
			QuestionID: questionID, Rank: 0, Reason: "full",
		})
		return nil, ErrRushFull
	}
	rank = int(res) + 1 // 0-based → 1-based
	bonus := 0 // 抢答奖励分已下线：抢答资格只决定谁能作答，得分一律走判分口径

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
	var p model.Participant
	e.DB.Where("quiz_id = ? AND user_id = ?", quizID, userID).First(&p)

	result = &ws.RushResultData{
		QuestionID: questionID, Rank: rank, Nickname: u.Nickname,
		Bonus: bonus, Score: p.Score,
	}
	e.Hub.SendToUser(quizID, userID, ws.EventRushSuccess, result)
	e.broadcastStatistics(quizID, questionID)

	// 名额满 → 不立刻关窗：至少保留 200ms，让慢半拍的选手也有"未抢到"的参与反馈
	// （rushEndLocked 幂等：原窗口计时器若先触发也只是空跑）
	if rank >= rt.quiz.RushWinnerCount {
		rt.startRushTimerLocked(200*time.Millisecond, func() { e.RushEnd(quizID) })
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
		nick2 := nick[r.UserID]
		if nick2 == "" {
			nick2 = "已退出用户"
		}
		out[i] = ws.RushWinner{UserID: r.UserID, Nickname: nick2, Rank: r.Rank, Bonus: r.Score}
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
			UserID: uid, Nickname: u.Nickname, Rank: i + 1, Bonus: 0,
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
