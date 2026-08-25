# E2E 测试用例清单

跑法（在仓库根目录）：

```bash
node scripts/security_e2e.mjs    # 理论答题安全（18 项）
node scripts/hardening_e2e.mjs   # 抢答并发/越权/防重复/重连（7 项）
```

> `security_e2e.mjs` 开头会清空 MySQL 各表 + Redis 并重启 server（`NO_CLEAN=1` 跳过），
> 两个脚本都会重建自己的测试数据，跑完自动清理，可重复执行。

## 总览

| # | 用例 | 脚本 | 状态 |
|---|------|------|------|
| A1 | 抢答模式开窗前普通提交被拒 | security_e2e | ✅ |
| A2 | 抢答成功 rank=1 | security_e2e | ✅ |
| A3 | 未抢到的选手提交被拒 | security_e2e | ✅ |
| A4 | 抢答窗口关闭后再抢被拒 | security_e2e | ✅ |
| A5 | 抢到资格的选手可提交 | security_e2e | ✅ |
| B1 | current-question 接口无答案 | security_e2e | ✅ |
| B2 | 即时 result 只回显本人答案 | security_e2e | ✅ |
| B4 | show_answer=false 时 reveal 无 correct_answer/analysis | security_e2e | ✅ |
| B5 | show_answer=true 时 reveal 单播含 correct_answer | security_e2e | ✅ |
| B6 | reveal 单播个人答案各拿各的 | security_e2e | ✅ |
| B7 | reveal 所有事件不含他人答案 | security_e2e | ✅ |
| C1 | 用户 token 调管理端 API 被拒 | security_e2e | ✅ |
| C2 | 未开始(WAITING)提交被拒 | security_e2e | ✅ |
| C3 | 未参加者提交被拒 | security_e2e | ✅ |
| C5 | 多选乱序提交（CA==AC）判对 | security_e2e | ✅ |
| C6 | 非法选项被拒 | security_e2e | ✅ |
| C7 | 结束(FINISHED)后提交被拒 | security_e2e | ✅ |
| C8 | 倒计时超时（含 1.5s 宽限）后提交被拒 | security_e2e | ✅ |
| H1 | 越权：跨 quiz token 提交被拒 | hardening_e2e | ✅ |
| H2 | 越权：未参加者提交被拒 | hardening_e2e | ✅ |
| H3 | 防重复：二次提交被拒且不重复加分 | hardening_e2e | ✅ |
| H4 | 答案不下发：current-question/info 无 answer/analysis | hardening_e2e | ✅ |
| H5 | 100 并发抢答：rank=1 唯一 | hardening_e2e | ✅ |
| H6 | 重连恢复：sync 带回状态与当前题 | hardening_e2e | ✅ |
| H7 | 重连后仍可作答 | hardening_e2e | ✅ |

## 用例详细说明

### 抢答权限（A 系列）—— scripts/security_e2e.mjs

- **A1 抢答模式开窗前普通提交被拒**：rush 模式 quiz start 后题目即进入 ANSWERING，
  在 admin 执行 rush/start 开窗之前，普通用户直接提交答案必须被拒
  （曾发现绕过漏洞：开窗前可答题绕过抢答，已修复——`flow.go` SubmitAnswer 对
  `mode=rush` 一律要求 RushRecord 资格）。断言：提交返回 code≠0。
- **A2 抢答成功 rank=1**：Alice 在抢答窗口内抢答，断言返回 `rank=1`。
- **A3 未抢到的选手提交被拒（核心）**：Bob 未抢到资格，对同一题提交答案被拒
  （msg=未获得本题答题资格）。这是抢答机制的核心安全断言。
- **A4 抢答窗口关闭后再抢被拒**：rush_time 到期后 Bob 再发起抢答，被拒。
- **A5 抢到资格的选手可提交**：Alice（rank=1）在 rush_answer_time 内提交成功，code=0。

### 答案回显（B 系列）—— scripts/security_e2e.mjs

- **B1 current-question 无答案**：题目下发接口/WS 事件的载荷中不能出现
  `answer` / `correct_answer` / `analysis` 字段（正则扫描整个 JSON）。
- **B2 即时 result 只回显本人答案**：提交答案后的 `answer:result` 单播只含本人
  `answer` 与得分，不含正确答案与解析。
- **B4 show_answer=false 时 reveal 无答案**：quiz 关闭 show_answer 时，收到的**所有**
  `answer:reveal` 事件（公共广播+单播）均无 `correct_answer`/`analysis`
  （曾发现泄露：adminData 全量广播给所有用户，已修复）。
- **B5 show_answer=true 时 reveal 单播含 correct_answer**：开启 show_answer 的 quiz，
  reveal 单播事件含 `correct_answer`。
- **B6 reveal 单播个人答案各拿各的**：同一题 Alice 答 B、Bob 答 A，各自 WS 收到的
  `my_answer` 与本人一致（不能看到对方答案）。
- **B7 reveal 所有事件不含他人答案**：Alice/Bob 收到的全部 reveal 事件里
  `my_answer` 只能是自己的或缺失（公共广播无个人字段）。

### 状态机 / 越权 / 倒计时（C 系列）—— scripts/security_e2e.mjs

- **C1 用户 token 调管理端 API 被拒**：用户答题 token 请求 `GET /api/admin/quizzes` 返回 401。
- **C2 未开始(WAITING)提交被拒**：quiz 未 start 时提交答案 code≠0。
- **C3 未参加者提交被拒**：未 join 的用户（eve）用自己 token 提交他人 quiz 的题，被拒。
- **C5 多选乱序判对**：正确答案 AC，提交 "CA"（乱序）仍判 `is_correct=true`。
- **C6 非法选项被拒**：提交选项 "Z"，返回 msg=答案选项不合法。
- **C7 结束后提交被拒**：quiz End 之后提交答案被拒。
- **C8 倒计时超时后提交被拒**：1 秒时限的题，等 4 秒（超时+宽限 1.5s）后提交被拒
  （到点服务端强制收卷，已记录“未答”）。

### 抢答并发 / 越权 / 防重复 / 重连（H 系列）—— scripts/hardening_e2e.mjs

- **H1 跨 quiz token 提交被拒**：A quiz 的答题 token 不能提交 B quiz 的题。
- **H2 未参加者提交被拒**：同 C3，hardening 场景复验。
- **H3 防重复**：同一题二次提交被拒（或不再加分），分数只累计一次
  （DB 唯一索引 + Redis 判重双保险）。
- **H4 答案不下发**：题目信息接口（REST+WS）无 answer/analysis 字段。
- **H5 100 并发抢答**：100 个连接同时抢 1 个名额，恰好 1 个 winner、99 个失败，
  无并列（Redis Lua + DB 唯一索引保证原子性）。
- **H6 重连恢复**：断线重连后 `sync` 事件带回 quiz 状态、当前题、剩余时间。
- **H7 重连后仍可作答**：重连的连接可以正常提交答案。

## Git 提交约束

**任何代码修改在 `git commit` 之前必须完整跑一遍上述两个脚本，全部通过才能提交。**

```bash
node scripts/security_e2e.mjs && node scripts/hardening_e2e.mjs
```
