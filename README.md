# 🎯 线上实时答题系统

实时答题活动系统：管理员创建答题、发布题目、控制流程；用户输入昵称加入、答题、抢答、查看实时排名。

> 需求全文见 [task.md](task.md)，开发计划见 [plan.md](plan.md)，贡献规范见 [AGENTS.md](AGENTS.md)

---

## 快速开始（Docker 一键部署）

```bash
docker compose up -d --build
```

启动 4 个容器：MySQL 8 / Redis 7 / Go 后端 / Vue3 前端(nginx)，首次启动自动建库建表。

## 访问入口

| 入口 | 地址 | 说明 |
|---|---|---|
| **答题用户端** | `http://<服务器IP>:13000/join` | 输入昵称 + 6位邀请码加入 |
| **管理端** | `http://<服务器IP>:13000/admin/login` | 默认账号 `admin` / `admin123` |
| 后端 API 直连 | `http://<服务器IP>:18080` | REST + WebSocket |

- 前端 nginx 已反代 `/api` 与 `/ws` 到后端，**任意 IP / 域名访问均可**，无需改配置
- 服务器需放通 `13000`（页面/API/WS）与 `18080`（可选，API 直连调试用）

## 测试流程（5 分钟跑通）

> 快速造数：`python3 scripts/gen_testdata.py http://<服务器IP>:13000`
> 自动创建「网络安全知识竞赛」：单选/多选/判断爻 20 题，每种题型前 10 题必答、后 10 题抢答（共 60 题，含解析），并打印邀请码

1. **管理端** `http://IP:13000/admin/login` 登录 → 创建答题（普通模式）→ 添加几道题（单选/多选/判断）
2. 复制列表中的 **邀请码**（6 位）
3. **用户端**（手机/电脑浏览器开 `http://IP:13000/join`）输入昵称 + 邀请码，可开多个不同昵称模拟多人
4. 管理端打开 **控制台** → `▶ 开始答题` → 用户端自动收到第 1 题
5. 用户提交答案 → 控制台实时看到 已答/正确/选项分布
6. `📢 公布答案` → 用户端显示正确答案与解析（受配置开关控制）
7. `下一题` … 最后一题后 `■ 结束答题` → 用户端显示成绩 + 最终排行榜

## 技术栈

- **前端**：Vue 3 + TypeScript + Vite + Pinia（用户端 / 管理端同工程）
- **后端**：Go (Gin + GORM + gorilla/websocket + go-redis)
- **存储**：MySQL 8（持久化）+ Redis 7（抢答原子操作，阶段5接入）
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
cd server && KAOSHI_ENV=dev go run ./cmd/server

# 前端（:5173，/api /ws 代理到 localhost:18080）
cd web && npm run dev
```

> 开发模式下 vite 代理指向 18080（docker 映射端口）；若本地直跑后端用 `KAOSHI_ADDR=:18080 go run ./cmd/server` 即可对上

## 端口与配置

| 端口(宿主机) | 服务 | 环境变量（server 容器） |
|---|---|---|
| 13306 | MySQL | `KAOSHI_MYSQL_DSN` |
| 16379 | Redis | `KAOSHI_REDIS_ADDR` |
| 18080 | Go 后端 | `KAOSHI_JWT_SECRET` / `KAOSHI_ADMIN_USER` / `KAOSHI_ADMIN_PASS` |
| 13000 | 前端(nginx) | — |

改密钥/账号：编辑 `docker-compose.yml` 中 `KAOSHI_*` 环境变量后 `docker compose up -d server`。

## 当前状态

✅ 阶段 0-4 完成：脚手架 / 数据模型 / 管理端 CRUD / 用户加入 + WS / 普通答题全流程（自动判分、实时统计、排行榜、管理控制台）

⏳ 待开发：阶段 5 抢答引擎（Redis Lua）→ 阶段 6 积分/排行榜细化 → 阶段 7 统计页 → 阶段 8 压测加固

## 数据重置

```bash
docker exec kaoshi-mysql mysql -uroot -proot123456 kaoshi \
  -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"
```
