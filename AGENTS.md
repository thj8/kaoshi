# AGENTS.md

面向 AI 编程助手（及新成员）的项目说明。改动前请先读完本文件。

## 项目简介

线上实时答题系统：管理员创建答题活动、发布题目、控制流程；用户账号登录进入、答题、抢答、查看实时排名。

- 需求全文见 `docs/task.md`
- 开发计划与阶段划分见 `docs/plan.md`，使用说明见 `README.md`
- 当前进度：阶段 0-8 ✅ 全部完成（抢答原子性、实时排行榜、统计页、加固 E2E 均通过；回归脚本 scripts/hardening_e2e.mjs）

## 仓库结构

```
kaoshi/
├── docker-compose.yml      # mysql(13306) redis(16379) server(18080) web(13000)
├── README.md / AGENTS.md
├── docs/                  # 需求、计划、用例、接口文档
│   ├── task.md / plan.md / TESTCASES.md
│   ├── API.md / design-quiz-access.md
│   └── 客户演示指南.md     # 不入库（含真实凭据，已 gitignore）
├── server/                 # Go 后端 (Gin + GORM + gorilla/websocket + go-redis)
│   ├── cmd/server/         # 入口 main.go
│   ├── internal/
│   │   ├── config/         # 环境变量配置
│   │   ├── model/          # GORM 模型（唯一数据模型定义处）
│   │   ├── handler/        # REST handler（api/ 用户端、admin/ 管理端）
│   │   ├── ws/             # WebSocket Hub/Client/消息协议
│   │   ├── engine/         # 状态机、判分、抢答、倒计时（核心业务）
│   │   ├── store/          # MySQL/Redis 封装
│   │   └── middleware/     # CORS、JWT
│   └── migrations/         # SQL 迁移（GORM AutoMigrate 兜底）
└── web/                    # Vue3 + TS + Vite 前端
    └── src/
        ├── user/           # 用户端页面
        ├── admin/          # 管理端页面
        ├── api/            # REST 封装（axios，统一 {code,msg,data} 响应）
        ├── ws/             # WS 客户端（心跳+指数退避重连+状态恢复）
        ├── stores/         # Pinia
        ├── router/         # 路由（/join 用户端，/admin 管理端）
        └── styles/         # 全局样式（暗色科技风）
```

## 常用命令

后端（在 `server/` 下）：

```bash
go build ./...              # 编译检查
go vet ./...                # 静态检查
go run ./cmd/server         # 本地启动（需设 KAOSHI_JWT_SECRET/KAOSHI_ADMIN_PASS，依赖本地 MySQL:13306 / Redis:16379，或先起 docker）
```

前端（在 `web/` 下）：

```bash
npm run dev                 # 开发服务器 :5173，/api 与 /ws 已代理到 localhost:18080
npm run build               # 类型检查 + 构建（vue-tsc + vite）
```

整体部署：`docker compose up -d --build`（首次会自动建库建表）。

生产/测试运行（外部机器访问）：

- 前端+反代入口：`http://<服务器IP>:13000`（nginx 已反代 `/api`、`/ws` 到后端，任意 IP/域名访问无需改配置）
- 后端 API 直连：`http://<服务器IP>:18080`
- 用户端 `/join`，管理端 `/admin/login`（admin，密码在 `.env` 的 `ADMIN_PASS`，不入库）
- 容器日志：`docker compose logs -f server` / `web`；重置测试数据见 README「数据重置」
- **测试数据保护（硬性约束）**：不要清除/重置数据库中的测试/模拟数据（包括 E2E 产生的数据），除非用户明确要求

## 环境注意事项（重要，容易踩坑）

- Go 安装在 `/usr/local/go/bin`，不在默认 PATH：`export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- Go 代理默认超时，已设置 `GOPROXY=https://goproxy.cn,direct`（写入 go env，勿改回）
- 全局 `~/.npmrc` 指向内网 Nexus 且配置了 `omit=dev`（npm 不装 devDependencies，会导致 vite/vue-tsc 缺失）。
  **`web/.npmrc` 里的 `omit=` 覆盖项必须保留**，删掉它 `npm install` 会静默装不全
- 本机 Node 24 / npm 11；`package.json` 中 typescript 固定 ~5.8，勿升级到 6.x（与 vue-tsc 冲突）
- 宿主机端口均做了偏移避免冲突：后端 18080、前端容器 13000 绑 0.0.0.0（外部可访问）；MySQL 13306、Redis 16379 仅绑 127.0.0.1（外部不可达，密码为随机值）
- **前端容器禁止写死 API 地址**：nginx 反代 `/api`、`/ws`，前端一律用相对路径（`VITE_API_BASE` 留空），否则外部 IP 访问会指向用户自己的 localhost

## 领域不变量（写代码必须遵守）

1. **服务端是唯一事实来源**：当前题目、状态、倒计时、得分、排名、抢答结果全部由服务端判定；客户端只渲染
2. **正确答案绝不下发**：`Question.Answer`、`Analysis` 用 `json:"-"` 剥离；仅 `answer:reveal` 且 quiz 开启 `show_answer` 时才发送
3. **防重复**：answers / rush_records 表的 `(quiz_id, question_id, user_id)` 唯一索引 + Redis 判重，双保险；分数只在首次提交时累加
4. **倒计时以服务器时间为准**：下发 deadline 时间戳 + 服务端定时广播剩余秒数；到点服务端强制收卷
5. **抢答原子性**：Redis Lua/ZSET 判序，按服务器收到时间排序，禁止使用客户端时间；`rank` 是 MySQL 8 保留字，SQL 中必须写 `` `rank` ``（反引号）
6. **WS 消息协议**：`{event, data, ts}`，事件名对齐 docs/task.md 二十二节（`activity:* / question:* / answer:* / rush:* / ranking:update / statistics:update`）
7. **状态机**：`WAITING / RUNNING / PAUSED / RUSHING / ANSWERING / REVEALING / FINISHED`，任何写操作先校验状态

## 编码约定

- Go：标准 Go 风格；handler 保持薄，业务逻辑放 `engine/`；错误用 `gin.H{"code":..., "msg":...}` 包装，code=0 成功
- **锁纪律**：`Runtime.mu` 不可重入。持锁路径只能调 `xxxLocked` 内部方法（如 `getOptionsLocked`），对外方法（如 `GetOptions`）自带锁——历史上因重入死锁过一次
- **GORM 零值陷阱**：`time.Time` 字段插入前必须显式赋值 `time.Now()`，否则 MySQL 拒收且错误被误判为唯一索引冲突
- **GORM bool 陷阱**：模型 bool 字段禁止加 `default` 标签——显式 false 是零值，GORM 会改写字段为默认值（default:true 时显式 false 变 true，极隐蔽）
- **管理端建题字段名**：REST 用 `time_limit`（秒），不是 `duration`——传错会静默落 0 导致题目无倒计时不强制收卷
- Vue：一律 `<script setup lang="ts">` 组合式 API；REST 调用走 `src/api/index.ts` 的 `http`/`unwrap`；token 存 localStorage（key 见 `LS` 常量）
- 新增页面：用户端放 `src/user/`；管理后台页面放 `src/admin/` 并作为 `AdminLayout` 的子路由注册（`src/router/index.ts`），侧边栏导航同步更新
- **加入方式**：账号由管理端「用户管理」创建（无自助注册接口）；用户在 `/login` 登录，再通过 `/join/<比赛码>` 链接自动 `POST /api/join {quiz_id: <比赛码>}` 换取答题作用域 token（含 quiz_id，供 WS 与答题接口鉴权）
- 样式用全局 CSS 变量（`src/styles/main.css`），移动端适配必须考虑（现场手机答题）

## Git 约定

- **提交前必须全量跑 E2E（硬性约束）**：任何代码修改在 `git commit` 前必须依次跑通，全部通过才能提交：

  ```bash
  node scripts/security_e2e.mjs && node scripts/hardening_e2e.mjs
  ```

- **E2E 执行时机**：仅在 git commit 前或用户明确要求时运行；平时修改代码/构建后不要主动跑 E2E
- 用例清单与详细说明见 `docs/TESTCASES.md`。任一断言失败：先修复再提交，禁止跳过或只跑其中一个。
- **分支工作流（重要）**：每个阶段一个分支开发，完成后再合并回 main
  1. 开发前：`git checkout main && git pull` → `git checkout -b feat/stageN-简短英文`（如 `feat/stage5-rush`、`feat/stage7-stats`；修复用 `fix/xxx`）
  2. 阶段内可多次提交，消息：`<type>: 阶段N 描述`，type 用 chore/feat/fix/docs
  3. 验证通过后：`git checkout main && git merge --no-ff feat/stageN-xxx`（保留合并记录）→ 删除分支
  4. main 只接收合并，不直接开发提交（文档小改除外）
- 提交信息用中文；修复类建议注明根因（如 `fix: 修复运行时死锁（重入锁拆分）`）
- 不提交 node_modules、构建产物、二进制资源（见 .gitignore）
