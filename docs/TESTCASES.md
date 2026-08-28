# E2E 测试用例清单

跑法（在仓库根目录）：

```bash
node scripts/security_e2e.mjs    # 理论答题安全（18 项）
node scripts/hardening_e2e.mjs   # 抢答并发/越权/防重复/重连/考试模式（39 项）
```

> `security_e2e.mjs` 开头会清空 MySQL 各表 + Redis 并重启 server（`NO_CLEAN=1` 跳过），
> 两个脚本都会重建自己的测试数据，跑完自动清理，可重复执行。

## 总览

| # | 用例 | 脚本 | 状态 |
|---|------|------|------|
| A1 | 抢答模式开窗前普通提交被拒 | security_e2e | ✅ |
| A2 | 抢答成功 rank=1 且无奖励分 | security_e2e | ✅ |
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
| C9 | 失效 token（用户已删）提交答案被拒 | security_e2e | ✅ |
| C10 | 失效 token（用户已删）抢答被拒 | security_e2e | ✅ |
| C11 | 到点宽限（1.5s）内补交被接受，收卷不覆盖 | security_e2e | ✅ |
| H1 | 越权：跨 quiz token 提交被拒 | hardening_e2e | ✅ |
| H2 | 越权：未参加者提交被拒 | hardening_e2e | ✅ |
| H3 | 防重复：二次提交被拒且不重复加分 | hardening_e2e | ✅ |
| H4 | 答案不下发：current-question/info 无 answer/analysis | hardening_e2e | ✅ |
| H5 | 100 并发抢答：rank=1 唯一 | hardening_e2e | ✅ |
| H6 | 重连恢复：sync 带回状态与当前题 | hardening_e2e | ✅ |
| H7 | 重连后仍可作答 | hardening_e2e | ✅ |
| H8 | 实时统计：已答不含未答占位行 | hardening_e2e | ✅ |
| H9 | 实时统计：正确/错误不含未答（1/1） | hardening_e2e | ✅ |
| H10 | 实时统计：正确率分母为真实作答（50%） | hardening_e2e | ✅ |
| S1 | 抢答答对=题目分值，无奖励分 | security_e2e | ✅ |
| S2 | 抢答答错按题型扣分（-4） | security_e2e | ✅ |
| S3 | 必答答对按题目分值、答错 0 分 | hardening_e2e | ✅ |
| S4 | rush 模式答对=题目分值无奖励 | security_e2e | ✅ |
| T1 | 即时 total_score 跨题累计 | hardening_e2e | ✅ |
| T2 | 排行榜按分数降序、名次连续 | hardening_e2e | ✅ |
| T3 | 总分=各题得分之和（20/10/0） | hardening_e2e | ✅ |
| T4 | 未答题者 0 分在榜 | hardening_e2e | ✅ |
| T5 | 成绩单总分=累计分 | hardening_e2e | ✅ |
| T6 | 过题防护：切题后提交已过去的题目被拒 | hardening_e2e | ✅ |
| T7 | 过题防护：切题后更改已答题目答案被拒 | hardening_e2e | ✅ |
| X1 | 普通模式混合题：开窗前直接提交被拒 | security_e2e | ✅ |
| X2 | 窗口开启中未抢先答被拒 | security_e2e | ✅ |
| X3 | 普通模式混合题抢答成功 rank=1 | security_e2e | ✅ |
| X4 | 混合题抢到后可提交 | security_e2e | ✅ |
| X5 | 名单设置与读取（含去重） | security_e2e | ✅ |
| X6 | 受邀者可加入 | security_e2e | ✅ |
| X7 | 未受邀者被拒(403)/brief公开 | security_e2e | ✅ |
| X8 | 受限赛对未受邀者不可见 | security_e2e | ✅ |
| X9 | 空名单开放可加入 | security_e2e | ✅ |
| X10 | RUNNING 改名单被拒 | security_e2e | ✅ |
| X11 | 不存在用户整单拒绝 | security_e2e | ✅ |
| X12 | 用户调名单接口被拒 | security_e2e | ✅ |
| X13 | 已加入标记 joined=true | security_e2e | ✅ |
| X14 | 我的比赛含进行中（+分数实时） | security_e2e | ✅ |
| X15 | 已结束在我的列表+分数 | security_e2e | ✅ |
| X16 | 参与者可重入/未参与者不可 | security_e2e | ✅ |
| X17 | 无 token 访问我的比赛被拒 | security_e2e | ✅ |
| E1 | 非法 mode 被拒（oneof） | hardening_e2e | ✅ |
| E2 | 考试模式创建 mode=exam | hardening_e2e | ✅ |
| E3 | 未开始(WAITING)不下发题目但含题数 | hardening_e2e | ✅ |
| E4 | 开考全卷下发：3题+截止时间 | hardening_e2e | ✅ |
| E5 | 试卷不含 answer/analysis | hardening_e2e | ✅ |
| E6 | 多选乱序归一化(CA→AC) | hardening_e2e | ✅ |
| E7 | 非法选项被拒 | hardening_e2e | ✅ |
| E8 | 空答案清除草稿（全取消=未答） | hardening_e2e | ✅ |
| E9 | 交卷前可改答案（A→B 生效） | hardening_e2e | ✅ |
| E10 | 并发首存同一题：均成功且仅一条 | hardening_e2e | ✅ |
| E11 | 考试模式屏蔽逐题/流程接口 | hardening_e2e | ✅ |
| E12 | 交卷判分：未答不算错 | hardening_e2e | ✅ |
| E13 | 交卷后修改答案被拒 | hardening_e2e | ✅ |
| E14 | 重复交卷幂等（同分） | hardening_e2e | ✅ |
| E15 | 答错0分（1对1错） | hardening_e2e | ✅ |
| E16 | 重连恢复：sync 回考试态 | hardening_e2e | ✅ |
| E17 | activity:end 广播排行榜（降序） | hardening_e2e | ✅ |
| E18 | 统一收卷：未交卷者按已存答案计分 | hardening_e2e | ✅ |
| E19 | 结束重算不改动已交卷成绩 | hardening_e2e | ✅ |
| E20 | 到时自动收卷+重算 | hardening_e2e | ✅ |
| E21 | 到时后保存被拒 | hardening_e2e | ✅ |

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
- **X1 普通模式混合题：开窗前直接提交被拒（核心）**：normal 模式 quiz 中
  `required=false` 的抢答题，在无人抢答（无任何 RushRecord）之前直接提交必须被拒。
  （曾发现绕过漏洞：资格校验以"该题已发生过抢答记录"判定抢答题，第一人抢答前
  任何人可直接作答；已修复——按题目属性 `required=false` 判定，而非是否已有人抢过。）
- **X2 窗口开启中未抢先答被拒**：rush/start 后未抢答者提交被拒。
- **X3 混合题抢答成功**：normal 模式下抢答返回 rank=1。
- **X4 混合题抢到后可提交**：rush/end 后持有 RushRecord 者提交成功。
- **X5-X17 邀请制 + 我的比赛**（quiz_invitees，TDD 先行）：
  名单非空=仅名单内用户可加入（join 403、`/api/quizzes` 不可见），为空=开放（回归）；
  仅 WAITING 可 PUT 名单，含不存在 user_id 整单拒绝，用户 token 调名单接口 401；
  `/api/quizzes` 带 `joined` 标记；`/api/my/quizzes` 含进行中（分数实时）与已结束
  （分数准确、未参加者不可见）；已结束比赛老参与者可重新 join 拿 token 回看成绩，
  未参与者被拒；无 token 访问 my 接口 401。

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
- **C11 到点宽限内补交被接受**：2 秒时限的题，到点后 ~0.3s 提交（模拟前端“时间到自动补交已选答案”）→
  判分成功；越过宽限收卷后查 result，该答案未被覆盖为“未答”（correct=1）。
  配套实现：服务端收卷定时器延后 1.5s（`forceCollect` 的 `collectGrace`，与提交宽限对齐），
  否则收卷先落库会把在途补交挡成“已提交过本题答案”。

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
- **H8 实时统计不含未答占位（已答）**：3 人必答题，2 人作答（1 对 1 错）、
  1 人超时由强制收卷补"未答"占位行（answer="-"），reveal 后题目统计
  `answered=2`——占位行不计入已答。
  （曾发现统计 bug：统计口径未排除占位行，已答虚高、未答=参与−已答出现负数、
  未答被算进错误数与正确率分母；已修复——`flow.go`/`stats.go` 统计一律
  加 `answer != "-"` 过滤。）
- **H9 实时统计正确/错误口径**：同场景 `correct=1`、`wrong=1`，未答不算错。
- **H10 正确率分母为真实作答**：`correct_rate=50%`（1/2），不是 1/3。

### 计分口径（S 系列）

> 计分口径：答对得本题分值（quiz 按题型配置优先，0=沿用题目自带分值）；
> 必答题答错 0 分；**抢答题答错倒扣**——按题型扣分配置（RushDeduct，0=未配置），
> 未配置时需 `rush_wrong_score>0` 总开关，扣本题分值；抢答成功本身无奖励分。

- **S1 抢答答对=本题分值，无奖励分**：security_e2e A 场景，题目 10 分、quiz 配置
  `rush_bonus_score=5`，抢到后答对 → `score=10, total=10`（不含 +5 奖励）。
- **S2 抢答答错按题型扣分**：quizX 配置 `rush_deduct_single=4`，第二题抢到后
  答错 → `score=-4, total=6`（首题答对 +10）。
- **S3 必答答对按题目分值、答错 0 分**：hardening_e2e 统计场景，答对者 +10、
  答错者 +0（超时未答同样 +0）。
- **S4 抢到的答对=题目分值无奖励（quizR）**：rush 模式复验，`score=10, total=10`。

### 成绩总分与排行榜（T 系列）—— scripts/hardening_e2e.mjs

场景：2 道必答题（各 10 分），3 人参加——u1 全对、u2 一错一对、u3 只加入不答题。

- **T1 即时 total_score 跨题累计**：第二题答对后 `answer:result` 的
  `total_score=20`（10+10）。
- **T2 排行榜按分数降序**：rank=1 分数 > rank=2 分数，名次连续。
- **T3 总分=各题得分之和**：三人分别为 20 / 10 / 0（错题 0 分、未答 0 分）。
- **T4 未答题者 0 分在榜**：只 join 未作答的用户以 0 分出现在排行榜。
- **T5 成绩单总分=累计分**：`GET /api/quiz/:id/result` 返回的 rank=1、
  答题 2 题、正确率 100%。
- **T6/T7 过题防护**：普通模式下主持人切到下一题后，对旧题再次提交（补交或改答案）
  均被拒（400「当前题目不匹配」）——服务端仅接受当前题，不允许回改已过去的题目。

- **C9/C10 失效身份提交被拒**：用户被管理端删除（级联 participants/answers）后，
  其旧答题 token 提交答案/抢答必须返回非 0（401 账号已失效 / 400 参赛信息不存在）。
  （曾发现严重 bug：SubmitAnswer 不校验参赛者存在，清库/重置后的旧 token 提交
  返回 code=0、score 正常、total_score=0，产生孤儿答案且总分永远为 0；
  已修复——UserAuth 中间件校验用户仍存在 + SubmitAnswer/RushSubmit 校验
  participant 行存在。）

### 考试模式：自由切题（E 系列）—— scripts/hardening_e2e.mjs

场景：exam 模式、总时长 120s、3 题（单选 B/10分、判断 A/10分、多选 AC/10分）。
三人参加：u1 乱序作答后交卷（q1=B 对、q3=AC 对、q2 未答）、u2 一对一错后交卷、
u3 只保存不交卷（并发首存）。另有 3s 时长小场验到时自动收卷。

- **E1 非法 mode 被拒**：`mode=free` 创建被拒（binding `oneof=normal rush exam`）。
- **E2 考试模式创建**：`mode=exam` 创建成功且回显。
- **E3 未开始不下发题目（防提前看题）**：WAITING 时 `GET /api/quiz/:id/paper`
  返回 `status=WAITING` 且 `questions=[]`，但 `question_count=3`（仅题数不含内容，
  供等待页展示「共 N 道题」）。
- **E4 开考全卷下发**：start 后试卷 `status=RUNNING`、3 题、`deadline_at>0`
  （= started_at + TotalTime，服务端唯一事实来源）。
- **E5 答案绝不下发**：试卷 JSON 无 `"answer":`/`"analysis":`
  （`"my_answer":` 不受影响，是本人草稿）。
- **E6 多选乱序归一化**：存 `CA` → 归一化为 `AC`（与正确答案比较前排序）。
- **E7 非法选项被拒**：存 `XZ` 被拒（选项 label 白名单）。
- **E8 空答案清除草稿**：存 `''` = 清除该题草稿（`my_answer` 回到 null，交卷时视为未答）。
  （曾发现 bug：Answer 带 `binding:"required"` 且引擎拒绝空串，多选题逐一取消到
  全空时前端保存报错——UI 已清空但服务端残留旧答案，交卷按旧答案计分；
  已修复——空串删除草稿记录。）
- **E9 交卷前可改答案**：q1 先存 A 再改 B，试卷回读 `my_answer=B`。
- **E10 并发首存同一题**：同一题两条并发保存均成功、只落一条记录
  （唯一索引 + 冲突退化更新），最终答案 ∈ {A, AC}。
- **E11 考试模式屏蔽逐题/流程接口**：`/api/question/:id/answer`、admin 的
  `next`/`pause`/`rush/start` 在考试模式全部被拒。
- **E12 交卷判分口径**：2 对（20 分）、`answered=2`、**未答不算错**（`wrong=0`）。
- **E13 交卷后锁定**：交卷后再保存答案被拒（“已交卷，不能再修改答案”）。
- **E14 重复交卷幂等**：再次 submit 返回 code=0 且同分（不重复计分）。
- **E15 答错 0 分**：u2 1 对 1 错 → 10 分、`wrong=1`。
- **E16 重连恢复考试态**：WS 重连后 `sync` 带回 `status=RUNNING`、
  `deadline_at>0`、`question=null`（考试无“当前题”）。
- **E17 收卷广播**：admin end 后 `activity:end` 携带 3 人排行榜、降序、第一名 20 分。
- **E18 统一收卷**：未交卷者按已保存答案计分（收卷时从 answers 重算全员）。
- **E19 结束重算不损伤已交卷成绩**：end 后 u1 仍 20 分、`finished=true`
  （End 的 `finished_at IS NULL` 补录不覆盖主动交卷时间戳）。
- **E20 到时自动收卷**：3s 小场答题后等到时，`finished=true`、按已存答案得 10 分
  （服务端 timer → End → 重算）。
- **E21 到时后保存被拒**：FINISHED 后再保存答案被拒。

## Git 提交约束

**任何代码修改在 `git commit` 之前必须完整跑一遍上述两个脚本，全部通过才能提交。**

```bash
node scripts/security_e2e.mjs && node scripts/hardening_e2e.mjs
```
