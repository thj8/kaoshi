# AGENTS.md

面向 AI 编程助手（及新成员）的项目说明。改动前请先读完本文件。

## 项目简介

线上实时答题系统：管理员创建答题活动、发布题目、控制流程；用户输入昵称进入、答题、抢答、查看实时排名。

- 需求全文见 `task.md`
- 开发计划与阶段划分见 `plan.md`
- 当前进度：阶段 0（脚手架）已提交；阶段 1+ 按计划推进

## 仓库结构

```
kaoshi/
├── docker-compose.yml      # mysql(13306) redis(16379) server(18080) web(13000)
├── task.md / plan.md
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
go run ./cmd/server         # 本地启动（依赖本地 MySQL:13306 / Redis:16379，或先起 docker）
```

前端（在 `web/` 下）：

```bash
npm run dev                 # 开发服务器 :5173，/api 与 /ws 已代理到 localhost:18080
npm run build               # 类型检查 + 构建（vue-tsc + vite）
```

整体部署：`docker compose up -d --build`（首次会自动建库建表）。

## 环境注意事项（重要，容易踩坑）

- Go 安装在 `/usr/local/go/bin`，不在默认 PATH：`export PATH=$PATH:/usr/local/go/bin:~/go/bin`
- Go 代理默认超时，已设置 `GOPROXY=https://goproxy.cn,direct`（写入 go env，勿改回）
- 全局 `~/.npmrc` 指向内网 Nexus 且配置了 `omit=dev`（npm 不装 devDependencies，会导致 vite/vue-tsc 缺失）。
  **`web/.npmrc` 里的 `omit=` 覆盖项必须保留**，删掉它 `npm install` 会静默装不全
- 本机 Node 24 / npm 11；`package.json` 中 typescript 固定 ~5.8，勿升级到 6.x（与 vue-tsc 冲突）
- 宿主机端口均做了偏移避免冲突：MySQL 13306、Redis 16379、后端 18080、前端容器 13000

## 领域不变量（写代码必须遵守）

1. **服务端是唯一事实来源**：当前题目、状态、倒计时、得分、排名、抢答结果全部由服务端判定；客户端只渲染
2. **正确答案绝不下发**：`Question.Answer`、`Analysis` 用 `json:"-"` 剥离；仅 `answer:reveal` 且 quiz 开启 `show_answer` 时才发送
3. **防重复**：answers / rush_records 表的 `(quiz_id, question_id, user_id)` 唯一索引 + Redis 判重，双保险；分数只在首次提交时累加
4. **倒计时以服务器时间为准**：下发 deadline 时间戳 + 服务端定时广播剩余秒数；到点服务端强制收卷
5. **抢答原子性**：Redis Lua/ZSET 判序，按服务器收到时间排序，禁止使用客户端时间
6. **WS 消息协议**：`{event, data, ts}`，事件名对齐 task.md 二十二节（`activity:* / question:* / answer:* / rush:* / ranking:update / statistics:update`）
7. **状态机**：`WAITING / RUNNING / PAUSED / RUSHING / ANSWERING / REVEALING / FINISHED`，任何写操作先校验状态

## 编码约定

- Go：标准 Go 风格；handler 保持薄，业务逻辑放 `engine/`；错误用 `gin.H{"code":..., "msg":...}` 包装，code=0 成功
- Vue：一律 `<script setup lang="ts">` 组合式 API；REST 调用走 `src/api/index.ts` 的 `http`/`unwrap`；token 存 localStorage（key 见 `LS` 常量）
- 新增页面：用户端放 `src/user/`，管理端放 `src/admin/`，并在 `src/router/index.ts` 注册懒加载路由
- 样式用全局 CSS 变量（`src/styles/main.css`），移动端适配必须考虑（现场手机答题）

## Git 约定

- 按阶段提交，消息格式：`<type>: 阶段N 描述`（如 `feat: 阶段4 普通答题流程与自动判分`），type 用 chore/feat/fix/docs
- 提交信息用中文；不提交 node_modules、构建产物、二进制资源（见 .gitignore）
