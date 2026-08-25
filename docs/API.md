# API 接口文档

后端基准地址：`http://<服务器IP>:18080`（前端经 `http://<服务器IP>:13000` 的 nginx 反代 `/api`、`/ws` 访问，前端一律用相对路径）。

统一响应格式（HTTP 状态码恒为 200，业务码在 body）：

```json
{ "code": 0, "msg": "", "data": { ... } }
```

- `code=0` 成功；非 0 失败（401 未登录 / 403 越权 / 400 业务错误 / 429 限速）
- 鉴权：`Authorization: Bearer <token>`。两种 token：
  - **管理端 token**：`POST /api/admin/login` 获得，`role=admin`
  - **用户全局 token**：`POST /api/auth/login` 获得，`role=user`（只能调登录/加入/me）
  - **答题 token**：`POST /api/join` 用全局 token 换取，含 `quiz_id` 作用域，用于答题/抢答/WS。**用户端答题接口全部需要它**

---

## 一、公开接口（无需登录）

### GET /api/health
健康检查。`data: {"status":"ok","time":<unix秒>}`

### POST /api/auth/login
用户登录（账号由管理员创建，无自助注册）。
```json
{ "username": "player1", "password": "player12345" }
```
`data: {"token":"<全局token>","user":{"id":1,"username":"player1","nickname":"选手一号"}}`
> 同 IP 连续 10 次失败锁 1 分钟（429）。

### POST /api/admin/login
管理端登录（同 IP 5 次失败锁 1 分钟）。
```json
{ "username": "admin", "password": "<.env 里的 ADMIN_PASS>" }
```
`data: {"token":"<管理token>"}`

### GET /api/quiz/:id/brief
活动公开信息（加入页展示）。`data: {"id","title","description","status","mode","participant_count"}`

---

## 二、用户端（需用户 token）

### GET /api/auth/me
当前用户信息。请求头带**全局 token**。

### POST /api/join
加入答题，全局 token 换答题作用域 token。请求头带**全局 token**。
```json
{ "quiz_id": 1 }
```
`data: {"token":"<答题token>","quiz":{...},"nickname":"..."}`

以下接口请求头均带**答题 token**（token 内绑定 quiz_id，跨 quiz 访问返回 403）。

### GET /api/quiz/:id
活动详情 + 参加人数 + 我的昵称。`:id` 必须与 token 内 quiz_id 一致。

### GET /api/quiz/:id/current-question
当前题目（刷新/断线恢复兜底）。**载荷不含答案/解析**。
`data: {"question":{"id","index","total","type","content","score","time_limit","options":[{"label","content"}]},"deadline_at":<ms>,"server_now":<ms>,"status":"ANSWERING"}`

### POST /api/question/:id/answer
提交答案（服务端判分，唯一事实来源；每题只能提交一次）。
```json
{ "answer": "AC", "duration": 3500 }
```
- 多选题乱序等价（"CA" == "AC"）；非法选项返回 400
- 状态校验：WAITING/FINISHED/REVEALING/非当前题均拒绝
- 倒计时以服务器 deadline 为准，超时（含 1.5s 网络宽限）拒绝
- 抢答题：必须有抢答记录（RushRecord），否则 400「未获得本题答题资格」
`data: {"is_correct":true,"score":10,"answer":"AC"}` —— **不含正确答案**（是否公布走 reveal）

### POST /api/question/:id/rush
抢答（仅 RUSHING 状态；Redis Lua 原子判序，rank=1..N 为名额内）。
`data: {"rank":1,"bonus_score":5,"server_time":<ns>}`；超名额/窗口关闭返回 400。

### GET /api/quiz/:id/ranking
实时排行榜。`data: {"items":[{"rank":1,"user_id":1,"nickname":"Alice","score":10,"correct":1}]}`

### GET /api/quiz/:id/result
个人成绩单（活动结束后）。含本人每题作答、得分、是否正确（正确答案仅在 quiz 开启 show_answer 时下发）。

---

## 三、管理端（需管理 token，前缀 /api/admin）

### 用户管理
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /users?keyword= | 用户列表（含参加场次/总分/正确率聚合） |
| POST | /users | 建号 `{username, password(≥10位), nickname}` |
| GET | /users/:id | 用户详情 |
| PUT | /users/:id | 改昵称/重置密码（均可选字段） |
| DELETE | /users/:id | 删除用户 |

### 答题 CRUD
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /quizzes | 活动列表 |
| POST | /quiz | 创建活动 |
| GET | /quiz/:id | 活动详情 |
| PUT | /quiz/:id | 更新活动 |
| DELETE | /quiz/:id | 删除活动 |
| POST | /quiz/:id/questions | 创建题目 |
| GET | /quiz/:id/questions | 题目列表（管理端可见答案） |
| PUT | /question/:qid | 更新题目 |
| DELETE | /question/:qid | 删除题目 |

创建活动 `POST /quiz` 主要字段：
```json
{ "title": "常识知识竞赛", "mode": "normal",       // normal | rush
  "per_question_time": 30,                          // 每题默认倒计时（秒）
  "rush_time": 10, "rush_answer_time": 15,          // 抢答窗口 / 抢到后答题时限（秒）
  "rush_bonus_score": 5,
  "show_answer": true, "show_analysis": true, "show_ranking": true }
```

创建题目 `POST /quiz/:id/questions` 主要字段：
```json
{ "type": "single",              // single 单选 | multiple 多选
  "content": "中国的首都？", "answer": "B", "analysis": "北京。",
  "score": 10, "required": true, "time_limit": 20,   // 注意字段是 time_limit（秒），不是 duration
  "options": [{"label":"A","content":"上海"},{"label":"B","content":"北京"}] }
```
> 正确答案/解析仅存在服务端，经 `json:"-"` 剥离，任何用户端接口不下发；仅 reveal 时按 quiz 开关裁剪下发。

### 流程控制（状态机 WAITING→RUNNING/ANSWERING→PAUSED/RUSHING/REVEALING→FINISHED）
| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /quiz/:id/start | 开始（发布第 1 题） |
| POST | /quiz/:id/pause | 暂停（冻结倒计时；抢答中不可暂停） |
| POST | /quiz/:id/resume | 继续 |
| POST | /quiz/:id/next | 下一题（最后一题后结束） |
| POST | /quiz/:id/previous | 上一题 |
| POST | /quiz/:id/reveal | 公布当前题答案 |
| POST | /quiz/:id/end | 结束活动 |
| POST | /quiz/:id/rush/start | 开启抢答窗口（进入 RUSHING） |
| POST | /quiz/:id/rush/end | 提前关闭抢答窗口 |
| GET | /quiz/:id/statistics | 实时/最终统计（正确率分布、逐题统计） |

---

## 四、WebSocket

**连接**：`ws://<host>/ws`，token 走 **`Sec-WebSocket-Protocol` 子协议**（不上 URL，避免进反代日志）：

```js
new WebSocket('ws://host/ws', [answerToken])       // 用户端（答题 token）
new WebSocket('ws://host/ws?quiz=1', [adminToken]) // 管理端（?quiz= 指定房间）
```

**消息格式**：`{"event":"<事件名>","data":{...},"ts":<unix秒>}`（事件名对齐 task.md 二十二节）

| 事件 | 方向 | 说明 |
|---|---|---|
| `activity:start / pause / resume / end` | 广播 | 活动生命周期 |
| `question:publish` | 广播 | 发布题目（含 deadline_at/server_now，**无答案**） |
| `question:countdown` | 广播 | 剩余秒数（服务器时间权威） |
| `question:force-collected` | 广播 | 到点强制收卷 |
| `answer:result` | 单播 | 本人提交结果（无正确答案） |
| `answer:reveal` | 广播+单播 | 公共事件无答案；本人单播含 my_answer/my_score/is_correct；correct_answer 仅 quiz 开 show_answer 且只进本人单播；管理端连接收全量（含 distribution） |
| `rush:start / rush:end` | 广播 | 抢答窗口开/关 |
| `rush:result` | 单播 | 本人抢答结果 rank |
| `rush:winner` | 广播 | 抢答成功者公布 |
| `ranking:update` | 广播 | 实时排行榜（quiz 开 show_ranking 才发） |
| `statistics:update` | 管理端 | 统计刷新 |
| `sync` | 单播（连接后） | 断线重连状态恢复（状态+当前题+剩余时间） |

客户端心跳：定时发 `{"event":"ping"}`，服务端回 `pong`；断线指数退避重连后靠 `sync` 恢复。

---

## 五、快速验证（curl）

```bash
B=http://<IP>:18080

# 管理登录（密码在 .env 的 ADMIN_PASS）
AT=$(curl -s $B/api/admin/login -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<ADMIN_PASS>"}' | jq -r .data.token)

# 建号 → 用户登录 → 加入
curl -s $B/api/admin/users -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' \
  -d '{"username":"u1","password":"player12345","nickname":"选手一"}'
GT=$(curl -s $B/api/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"u1","password":"player12345"}' | jq -r .data.token)
UT=$(curl -s $B/api/join -H "Authorization: Bearer $GT" -H 'Content-Type: application/json' \
  -d '{"quiz_id":1}' | jq -r .data.token)

# 答题
curl -s $B/api/question/1/answer -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' \
  -d '{"answer":"B","duration":1200}'
```
