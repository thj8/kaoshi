# 线上实时答题系统 — 开发计划

> 基于 task.md 制定。技术栈：Vue3 + TS + Vite / Go / MySQL / Redis / WebSocket / Docker Compose

---

## 一、总体架构

```
┌─────────────┐        ┌─────────────┐
│  用户端 (Vue) │        │ 管理端 (Vue)  │   同一个前端工程，两个路由模块
└──────┬──────┘        └──────┬──────┘
       │  HTTP(REST) + WebSocket(wss)   │
       └──────────┬────────────┘
                  ▼
          ┌──────────────┐
          │   Go 后端     │
          │  ┌────────┐  │
          │  │ REST API│  │  Gin
          │  │ WS Hub  │  │  gorilla/websocket，房间(quiz)维度广播
          │  │ 状态机   │  │  quiz 状态由服务端内存+Redis 维护
          │  │ 判分引擎 │  │
          │  │ 抢答引擎 │  │  Redis Lua 保证原子性
          │  └────────┘  │
          └───┬──────┬───┘
              ▼      ▼
           MySQL    Redis
          (持久化)  (状态/抢答/排行榜 ZSET)
```

核心原则：
- **服务端是唯一事实来源**：当前题目、状态、倒计时、得分、排名全部由服务端判定，客户端只做展示
- **正确答案绝不提前下发**：发布题目时剥离 answer/analysis 字段

---

## 二、目录结构规划

```
kaoshi/
├── docker-compose.yml          # mysql + redis + server + web
├── plan.md
├── task.md
├── server/                     # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/             # 配置
│   │   ├── model/              # GORM 模型
│   │   ├── handler/            # REST handler (api, admin)
│   │   ├── ws/                 # Hub / Client / 消息协议
│   │   ├── engine/             # 答题状态机、判分、抢答、倒计时
│   │   ├── store/              # MySQL / Redis 封装
│   │   └── middleware/         # JWT 鉴权、CORS、日志
│   └── migrations/             # SQL 迁移
└── web/                        # Vue3 + TS + Vite
    └── src/
        ├── user/               # 用户端页面
        ├── admin/              # 管理端页面
        ├── api/                # REST 封装
        ├── ws/                 # WS 客户端（心跳+重连）
        └── stores/             # Pinia 状态
```

---

## 三、开发阶段（按优先级，MVP 优先）

### 阶段 0：项目脚手架（0.5 天）
- [ ] docker-compose.yml：mysql 8 / redis 7 / server / web
- [ ] Go 项目初始化（Gin + GORM + gorilla/websocket + go-redis + jwt）
- [ ] Vite + Vue3 + TS + Pinia + Vue Router 初始化
- [ ] 配置读取、日志、热重载（air / vite dev proxy）

### 阶段 1：数据库与核心模型（0.5 天）
- [ ] 建表（按 task.md 二十四节）：
  - `users(id, nickname, created_at)`
  - `quizzes(id, title, description, status, mode, invite_code, 配置项..., total_time, created_at, started_at, ended_at)`
  - `questions(id, quiz_id, type[single|multiple|judge], content, answer, analysis, score, required, sort, time_limit)`
  - `question_options(id, question_id, label, content, sort)`
  - `participants(id, quiz_id, user_id, score, correct_count, wrong_count, joined_at)`
  - `answers(id, quiz_id, question_id, user_id, answer, is_correct, score, duration, submitted_at)`
  - `rush_records(id, quiz_id, question_id, user_id, server_time, rank, score, created_at)`
- [ ] GORM 模型 + 自动迁移 / SQL 文件
- [ ] 管理员账号（简单 JWT 登录即可，不做复杂权限）

### 阶段 2：管理端基础 CRUD（1 天）
- [ ] `POST /api/admin/quiz` 创建答题（生成邀请码、链接）
- [ ] `PUT /api/admin/quiz/:id` 修改配置（答题模式/抢答开关/每题时间/是否显示答案等）
- [ ] 题目 CRUD：单选 / 多选 / 判断 三种题型，分值/必答/答题时间/解析
- [ ] 管理端页面：答题列表、创建表单、题目编辑器
- [ ] 管理端登录 + JWT

### 阶段 3：用户进入 + WebSocket 基础设施（1 天）
- [ ] `POST /api/join`：昵称 + 邀请码 → 返回 user token（JWT，含 user_id/quiz_id）
- [ ] `GET /api/quiz/:id`：答题信息（名称/规则/参与人数/状态）
- [ ] WS Hub：按 quiz_id 分房间，管理员房间 + 用户房间
- [ ] WS 鉴权（连接时校验 JWT）、心跳 ping/pong
- [ ] 断线自动重连 + **重连状态恢复**（服务端按 quiz 状态下发当前题目、剩余时间、个人得分）
- [ ] 消息协议定义（对齐 task.md 二十二节的事件名）

### 阶段 4：普通答题流程（MVP 核心，1.5 天）
- [ ] 状态机：`WAITING → RUNNING → PAUSED → RUSHING/ANSWERING → REVEALING → FINISHED`
- [ ] `POST /api/admin/quiz/:id/start | pause | next | previous | end | reveal`
- [ ] 发布题目：`question:publish` 广播（**剥离答案**），题号、倒计时截止时间戳（服务器时间）
- [ ] 服务器倒计时：`question:countdown` 定期广播剩余时间；到时自动收卷
- [ ] `POST /api/question/:id/answer`：防重复提交（Redis SETNX / 唯一索引）、后端判分、多选全对才算对、必答/非必答逻辑
- [ ] `answer:result` 单播个人结果（是否正确由 reveal 配置控制展示答案内容）
- [ ] 管理员控制台：左题目列表 / 中当前题 / 右实时统计（参与、已答、未答、正确数、选项分布 A/B/C/D、组合答案分布）
- [ ] 用户端答题页：顶部信息栏（题号/得分/倒计时）、选项大按钮、上一题/下一题/提交、移动端适配

### 阶段 5：抢答引擎（1 天）
- [ ] 管理员 `rush/start` → 状态 RUSHING，广播 `rush:start`
- [ ] `POST /api/question/:id/rush`：**Redis Lua 脚本**原子判序（ZADD + 首次排名 / SETNX），只按服务器收到时间
- [ ] 按配置取前 N 名抢答成功，其余 `rush:failed`；唯一第一名、不可重复抢、不可重复得分
- [ ] 抢答成功者获得答题资格（可配置抢答奖励分），进入答题倒计时
- [ ] 抢答窗口（如 10 秒）结束广播 `rush:end`
- [ ] 用户端大号抢答按钮：灰(等待)/强调色(可抢)/绿(成功)/红(失败) 四态

### 阶段 6：积分与实时排行榜（1 天）
- [ ] 积分规则：普通题=题目分值；抢答题=抢答奖励 + 答对分 − 答错扣分（配置化）
- [ ] Redis ZSET 维护 quiz 排行榜，分数变动即 `ranking:update` 广播（排名/昵称/分数/正确题数）
- [ ] 用户端实时排行榜组件（WS 推送，不刷新页面）
- [ ] 公布答案 `answer:reveal`：显示正确答案/我的答案/得分/解析（受 quiz 配置开关控制）

### 阶段 7：结束与统计（1 天）
- [ ] `activity:end` → FINISHED，用户端成绩页：总分/题数/正确/正确率/用时/当前排名/最终排行榜
- [ ] `GET /api/quiz/:id/result`
- [ ] `GET /api/admin/quiz/:id/statistics`：参与数/完成数/平均分/最高最低分/平均正确率
- [ ] 题目维度统计：答题人数/正确率/平均用时 → 发现难题
- [ ] 管理端统计页面

### 阶段 8：加固与收尾（1 天）
- [ ] 安全检查：JWT/WS 鉴权、只能提交自己答案、防重复提交计分、答案不下发
- [ ] 并发测试：模拟 100 并发抢答，验证唯一性与稳定性
- [ ] 断线重连/刷新恢复完整回归
- [ ] UI 打磨（科技感、强反馈、移动端）、错误提示、空态
- [ ] Docker Compose 一键启动跑通全流程
- [ ] README（启动方式、演示流程）

---

## 四、关键技术设计

### 1. 状态机（服务端维护，Redis + 内存）
`WAITING / RUNNING / PAUSED / RUSHING / ANSWERING / REVEALING / FINISHED`
- 每次状态变更持久化到 quizzes.status，并广播对应 `activity:*` 事件
- 客户端任何操作先校验服务端状态（如非 ANSWERING 状态的提交直接拒绝）

### 2. 倒计时（服务器时间为准）
- 发布题目时下发 `deadline_at`（服务器时间戳）+ 定期广播剩余秒数 `question:countdown`
- 客户端只做展示渲染；服务端到点强制收卷（未答必答=记录未答，非必答=跳过）

### 3. 抢答原子性（Redis Lua）
```
KEY: quiz:{id}:q:{qid}:rush   (ZSET member=user_id score=server_ns)
Lua: ZCARD < N 且 ZSCORE user == nil → ZADD，返回排名；否则失败
```
- 100 并发下第一名唯一；`SETNX quiz:{id}:q:{qid}:answered:{uid}` 防重复得分

### 4. 防重复提交
- `answers` 表 (quiz_id, question_id, user_id) 唯一索引 + Redis 快速判重，双保险

### 5. 断线重连
- WS 客户端：指数退避重连 + ping/pong 心跳
- 重连后 `sync` 消息：服务端下发 quiz 状态 + 当前题(无答案) + deadline + 个人进度/得分 → 刷新页面不丢记录

### 6. 消息协议（对齐 task.md）
```json
{ "event": "question:publish", "data": { ... }, "ts": 1710000000 }
```
事件清单：activity:start/pause/end、question:publish/next/previous/countdown、
answer:submit/result/reveal、rush:start/submit/success/failed/end、
ranking:update、statistics:update

---

## 五、里程碑

| 里程碑 | 内容 | 验收标准 |
|---|---|---|
| M1（阶段0-2） | 脚手架 + 建模 + 管理 CRUD | 能创建答题、录题 |
| M2（阶段3-4） | 进入 + WS + 普通答题 | 完整走通普通模式，自动判分 |
| M3（阶段5-6） | 抢答 + 排行榜 | 100 并发抢答第一名唯一，排行榜实时刷新 |
| M4（阶段7-8） | 统计 + 加固 + 部署 | Compose 一键启动，全流程可用 |

预计总工时：约 7~8 个工作日。

---

## 六、MVP 优先级（task.md 第三十节）

```
管理员创建答题 → 用户进入 → 发布题目 → 用户答题 → 自动判分 → 抢答 → 实时排名 → 结束
     ✅阶段2        ✅阶段3     ✅阶段4      ✅阶段4     ✅阶段4    ✅阶段5   ✅阶段6   ✅阶段7
```

先纵向跑通主链路，再横向补全（暂停/上一题/解析开关/统计细化等）。
