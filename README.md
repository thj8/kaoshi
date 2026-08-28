# 🎯 线上实时答题系统

实时答题活动系统：管理员创建答题、发布题目、控制流程；用户账号登录加入、答题、抢答、查看实时排名。

> 需求全文见 [docs/task.md](docs/task.md)，开发计划见 [docs/plan.md](docs/plan.md)，贡献规范见 [AGENTS.md](AGENTS.md)，客户演示说明见 [docs/demo-guide.md](docs/demo-guide.md)

---

## 快速开始（Docker 一键部署）

```bash
cp .env.example .env   # 把 .env 里的密码改成随机值（不入库，git 已忽略）
docker compose up -d --build
```

所有凭据（MySQL/Redis 密码、JWT 密钥、管理端密码）都在 `.env` 中，compose 自动读取。

启动 4 个容器：MySQL 8 / Redis 7 / Go 后端 / Vue3 前端(nginx)，首次启动自动建库建表。

## 访问入口

| 入口 | 地址 | 说明 |
|---|---|---|
| **用户登录** | `http://<服务器IP>:13000/login` | 用户名 + 密码（账号由管理员在「用户管理」创建，无自助注册） |
| **答题用户端** | `http://<服务器IP>:13000/join/<比赛码>` | 比赛码为 10 位随机码，登录后自动加入 |
| **管理端** | `http://<服务器IP>:13000/admin/login` | 账号 `admin`，密码见 `.env` 的 `ADMIN_PASS` |
| 后端 API 直连 | `http://<服务器IP>:18080` | REST + WebSocket |

- 前端 nginx 已反代 `/api` 与 `/ws` 到后端，**任意 IP / 域名访问均可**，无需改配置
- 服务器需放通 `13000`（页面/API/WS）与 `18080`（可选，API 直连调试用）

## 管理后台

侧边栏布局（移动端自动折叠为顶部导航）：

- **答题管理**：活动列表 / 创建 / 题目编辑 / 控制台；每个活动提供「加入链接」一键复制发给用户
- **用户管理**：用户列表（用户名/昵称/参加场次/总得分/正确率）、搜索（用户名或昵称）、**新增用户**、**编辑（改昵称/重置密码）**、参加明细、删除（级联清理参与/答题/抢答记录）

## 测试流程（5 分钟跑通）

> 快速造数：`node scripts/gen_exam.mjs`（直连后端 18080，自动清库并创建「计分规则验证赛」）
> 自动创建 30 题验证赛（必答 40 分 + 抢答 60 分含倒扣，含解析与 5 个演示账号），并打印比赛码

1. **管理端** `http://IP:13000/admin/login` 登录 → 创建答题（普通模式）→ 添加几道题（单选/多选/判断）
2. 复制列表卡片上的 **加入链接**（`/join/<比赛码>`）发给用户
3. **用户端**账号由管理端「用户管理」创建，用户在 `/login` 登录后经 `/join/<比赛码>` 进入答题；可多浏览器/无痕窗口模拟多人
4. 管理端打开 **控制台** → `▶ 开始答题` → 用户端自动收到第 1 题
5. 用户提交答案 → 控制台实时看到 已答/正确/选项分布
6. `📢 公布答案` → 用户端显示正确答案与解析（受配置开关控制）
7. `下一题` … 最后一题后 `■ 结束答题` → 用户端显示成绩 + 最终排行榜

## 文档

- **docs/API.md** —— REST / WebSocket 完整接口文档（含 curl 快速验证）
- docs/TESTCASES.md —— E2E 测试用例清单（提交前必须全绿）
- `node scripts/seed.mjs` —— 清库并生成演示数据（3 个选手账号 + 2 场答题）

## 技术栈

- **前端**：Vue 3 + TypeScript + Vite + Pinia（用户端 / 管理端同工程）
- **后端**：Go (Gin + GORM + gorilla/websocket + go-redis)
- **存储**：MySQL 8（持久化）+ Redis 7（抢答原子判序，Lua 脚本保证并发下前 N 名唯一）
- **部署**：Docker Compose

## 架构要点

```
浏览器 ──HTTP/WS──> nginx(web:13000) ──反代──> Go server(:8080内网)
                                              ├── MySQL（业务数据/判分/排行）
                                              └── Redis（抢答原子判序）
```

- **服务端是唯一事实来源**：当前题目、状态机、倒计时、得分、排名全部由服务端判定
- **答案零泄漏**：`question:publish` 剥离 answer/analysis；仅 `answer:reveal` 且开启配置时下发
- **服务器倒计时**：下发 deadline 时间戳 + 每秒广播剩余秒；到点强制收卷（必答记未答）
- **防重复**：`(quiz_id, question_id, user_id)` 唯一索引 + 提交前查询双保险
- **断线重连**：WS 心跳 + 指数退避重连；重连即下发 `sync` 全量快照恢复进度

## 本地开发

```bash
# 依赖（或使用已运行的 docker 容器）
docker compose up -d mysql redis

# 后端（:8080，注意 go 在 /usr/local/go/bin）
cd server && KAOSHI_ENV=dev KAOSHI_JWT_SECRET=dev-only-secret KAOSHI_ADMIN_PASS=dev-only-pass go run ./cmd/server

# 前端（:5173，/api /ws 代理到 localhost:18080）
cd web && npm run dev
```

> 开发模式下 vite 代理指向 18080（docker 映射端口）；若本地直跑后端用 `KAOSHI_ADDR=:18080 KAOSHI_JWT_SECRET=dev-only-secret KAOSHI_ADMIN_PASS=dev-only-pass go run ./cmd/server` 即可对上

## 端口与配置

| 端口(宿主机) | 服务 | 环境变量（server 容器） |
|---|---|---|
| 13306 | MySQL（仅本机 127.0.0.1） | `KAOSHI_MYSQL_DSN` |
| 16379 | Redis（仅本机 127.0.0.1） | `KAOSHI_REDIS_ADDR` / `KAOSHI_REDIS_PASS` |
| 18080 | Go 后端 | `KAOSHI_JWT_SECRET` / `KAOSHI_ADMIN_USER` / `KAOSHI_ADMIN_PASS` |
| 13000 | 前端(nginx) | — |

改密钥/账号：编辑 `docker-compose.yml` 中 `KAOSHI_*` 环境变量后 `docker compose up -d server`。

> 安全：JWT secret 已随机化且为空即拒绝启动；admin 密码已随机化（拒绝空/弱默认启动）；MySQL/Redis 仅绑 127.0.0.1 且复杂密码；管理端登录同 IP 5 次失败锁 1 分钟；WS token 走 `Sec-WebSocket-Protocol` 头（不再进 URL/访问日志）。

## 当前状态

✅ 阶段 0-8 全部完成：脚手架 / 数据模型 / 管理端 CRUD / 用户加入 + WS / 普通答题全流程 / 抢答引擎（Redis Lua 原子判序）/ 实时排行榜 + reveal 个人答案单播 / 结束统计页 / 加固验证

### 实时排行榜与公布答案（阶段 6）

- 分数变动（提交 / 抢答 / 公布 / 抢答结束）即推 `ranking:update`，用户端浮动排行榜不刷新实时更新（受「显示排行榜」开关控制）
- 公布答案：正确答案/解析按开关裁剪广播，每人**单播**收到自己的答案/得分/对错；管理端额外看选项分布

### 统计页（阶段 7）

管理端答题列表 → 「统计」：参与/完成/平均分/最高最低分/平均正确率总览，题目正确率进度条（发现难题），完整排行榜；进行中 5s 自动刷新。用户结束后自动出成绩页（总分/答对答错/正确率/用时/排名/最终排行榜）。

### 加固验证（阶段 8）

```bash
node scripts/hardening_e2e.mjs   # 需先 docker compose up；BASE_URL 可覆盖地址
```

覆盖：跨 quiz 越权 / 未参加提交 / 重复提交只计一次分 / 答案绝不下发 / **100 并发抢答 rank=1 唯一** / WS 断线重连 sync 恢复状态后可继续作答。

### 抢答玩法（阶段 5）

1. 管理端创建活动时选「抢答模式」，可配置：获答名额 / 抢答窗口秒数 / 获答者答题秒数 / 抢答奖励分 / 答错扣分
2. 控制台发布题目后点「⚡ 开始抢答」→ 全员看到大号抢答按钮，按**服务器收到时间**排序取前 N 名
3. 获答者立即得到奖励分并进入专属答题倒计时；答错扣分（总分可为负）；未答由服务端强制收卷记未答
4. 窗口到时或名额满自动结束；管理员也可手动「结束抢答」；无人抢答可重新开抢

## 数据重置

```bash
docker exec kaoshi-mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" kaoshi \
  -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE quiz_invitees; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"
```
