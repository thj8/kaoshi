# 设计：比赛邀请制 + 我的比赛

> 状态：待确认。确认后开工，预计改动：后端 ~150 行、前端 ~250 行、E2E +3 用例。

## 1. 背景与目标

现状问题：

1. 任何登录用户都能加入任何比赛（`POST /api/join` 无门槛），无法控制「这场考试只给某些人」
2. 比赛一结束，用户端就再也找不到入口（`/api/quizzes` 只返回 WAITING），看不到自己参加过的比赛和成绩

目标：

- **A. 邀请制**：管理端创建比赛后可勾选参赛用户；只有被选中的用户能加入
- **B. 我的比赛**：用户端能看到自己加入过的所有比赛（含已结束），已结束的可点进去看成绩/排行榜

## 2. 数据模型

新表 `quiz_invitees`（GORM AutoMigrate，无需手写迁移）：

```go
type QuizInvitee struct {
    ID        int64     `gorm:"primaryKey;autoIncrement"`
    QuizID    int64     `gorm:"notNull;uniqueIndex:idx_invitee"`
    UserID    int64     `gorm:"notNull;uniqueIndex:idx_invitee"`
    CreatedAt time.Time
}
```

**规则（刻意不加开关字段）**：名单**非空** = 仅名单内用户可加入；名单**为空** = 开放所有人（与现状完全一致，老数据零迁移）。

> 备选方案：加 `Quiz.Restricted bool` 显式开关。弃选原因：多一个字段、多一个开关 UI、多一种「开关开了但名单忘了填」的出错态。隐式规则唯一代价：想开放就保持名单为空，直观。

## 3. 后端改动

### 3.1 管理端 API（新增 2 个）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/quiz/:id/invitees` | 返回 `{items: [{user_id, username, nickname}]}` |
| PUT | `/api/admin/quiz/:id/invitees` | 入参 `{user_ids: []}`，**全量替换**（幂等，前端整个列表一次提交） |

约束：

- 仅 WAITING 状态可改名单（与「已 start 不能建题」同一状态纪律，避免比赛中途加人/换人引入判分口径问题）
- user_ids 中的用户必须存在，忽略重复 id

### 3.2 加入校验（改 `Join`，auth.go）

现顺序不变，插入两条规则：

```
quiz 不存在                        → 404（现状）
受限（名单非空）且我不在名单        → 403 "该比赛未对你开放"
status == FINISHED：
    我已是参与者                   → 发答题作用域 token（用于回看成绩/排行，不新建 participant）
    我未参加                       → 400 "答题已结束"（现状）
其余                               → 幂等加入 + token（现状）
```

> FINISHED 重新发 token 是 B 功能的关键：用户清了缓存/换了设备也能从「我的比赛」点回已结束的比赛看成绩。答题/抢答接口有状态机校验，拿到 token 也提交不了任何东西，无风险。

### 3.3 我的比赛 API（新增 1 个）

`GET /api/my/quizzes`（用户全局 token）→ 按参与记录返回：

```json
{ "items": [{
    "quiz_id": 1, "title": "计分规则验证赛", "status": "FINISHED", "mode": "normal",
    "score": 85, "correct": 20, "wrong": 5, "joined_at": "...",
    "participant_count": 5
}] }
```

排序：进行中/等待中在前（started 优先），已结束按结束时间倒序。单条 SQL：`participants JOIN quizzes WHERE user_id=?`。

### 3.4 可加入列表（改 `QuizList`）

- 过滤：受限且我未受邀的比赛**不出现**在 `/api/quizzes`
- 每项增加 `joined: bool`（我是否已加入），前端显示「已加入」角标

`/api/quiz/:id/brief` 保持公开现状（标题/人数不含敏感信息，不泄露名单）。

## 4. 前端改动

### 4.1 管理端 — QuizEditPage（编辑页）

配置区新增「参赛用户」块，交互（已确认）：

- **全部用户列表**：每行前置复选框（用户名 + 姓名）+ 顶部搜索框（按用户名/姓名过滤）
- **全选**：控制当前过滤结果的勾选/取消（不影响已加入名单）
- **「加入」按钮**：把当前勾选的用户并入右侧已选名单（去重）
- **已选名单**：姓名/用户名 + 单个移除 ✕ + 清空 + 人数徽标
- **保存**：与其它配置一并 PUT 全量提交；状态非 WAITING 时整块只读

（创建对话框不加——创建后进编辑页设置，少一处状态同步）

### 4.2 用户端 — JoinPage 改为两个区块

```
┌ 我参加过的比赛 ────────────────────────┐
│ [进行中] 计分验证赛   我的分 40  进入→  │  → /quiz/:id
│ [已结束] 安全月考      我的分 85  成绩→ │  → 先 join 换 token 再 /rank/:id
└──────────────────────────────────────┘
┌ 可加入的比赛 ──────────────────────────┐
│ 网络知识赛  12人  [已加入|加入]         │
└──────────────────────────────────────┘
```

- 已结束的行：点击 → `POST /api/join`（换新 token 存 localStorage）→ 跳 `/rank/:id`
- 未登录时只显示「可加入」区（现状），登录后两区都显示

## 5. 边界情况

| 场景 | 行为 |
|---|---|
| 名单为空 | 开放所有人（现状） |
| 受限比赛，受邀者已加入后管理员将其移出名单 | 状态机限制：已 start 不能改名单 → 不存在此态（WAITING 中移出则其 join 被拒，已建的 participant 随 start 前重加入覆盖，无孤儿） |
| 管理员删除用户 | 现有级联删除（含 participants/answers）照旧；invitee 行随 user 级联删 |
| `/join/<id>` 直链访问受限比赛 | brief 可看标题，点「加入」时 403 提示「该比赛未对你开放」 |
| E2E 清库后 | 名单表同被清空，重建后开放，无残留 |

## 6. 测试计划（TDD：先写用例跑红 → 实现 → 全绿）

用例追加在 `scripts/security_e2e.mjs`（X 系列延续），覆盖全部边界：

| 用例 | 断言 |
|---|---|
| X5 名单设置与读取 | admin PUT 设名单 → GET 返回正确的 user_id/username/nickname；重复 id 幂等去重 |
| X6 受邀者可加入 | 名单内用户 join → code=0 拿到 token |
| X7 未受邀者被拒 | 名单外用户 join → code=403；直链 brief 仍可看标题（公开不泄露） |
| X8 未受邀者列表不可见 | 名单外用户 `/api/quizzes` 不含该比赛；名单内用户可见 |
| X9 开放回归 | 名单为空的比赛任何人可加入（老路径不破） |
| X10 状态限制 | RUNNING 后 PUT 名单 → code≠0；WAITING 才可改 |
| X11 不存在用户 | PUT 含不存在 user_id → code≠0，整单拒绝，原名单不变 |
| X12 越权 | 用户 token 调 admin invitees 接口 → 401 |
| X13 joined 标记 | 已加入用户在 `/api/quizzes` 中该比赛 `joined=true` |
| X14 我的比赛-进行中 | 参加中比赛出现在 `/api/my/quizzes`，score 实时 |
| X15 我的比赛-已结束 | 结束比赛在列且 score 正确；未参加者列表无此比赛 |
| X16 FINISHED 重入 | 已参与者 join 已结束比赛 → code=0（回看成绩）；未参与者 → code≠0 |
| X17 my 越权 | 无 token 访问 `/api/my/quizzes` → 401 |

TESTCASES.md 同步补行；提交前 security + hardening 全绿（硬约束）。

## 7. 不做的事（YAGNI）

- 不做按部门/批量导入名单（现场规模用不上，需要时再说）
- 不做「已 start 后补人」（迟到者）——状态纪律优先；真有迟到需求再开放
- 不做邀请链接/口令——需求是「选用户」，不是「发链接」
- 管理端创建弹窗不加选人步骤，编辑页统一入口
