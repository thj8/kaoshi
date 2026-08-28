# 线上实时答题系统

> 注：本文为原始需求文档。实现与需求的主要差异：加入方式由「昵称 + 邀请码」演进为「账号密码登录（管理端建号）+ 比赛码加入」，对外寻址统一用 10 位比赛随机码，详见 [README.md](../README.md) 与 [API.md](API.md)。

请开发一个完整的线上实时答题系统，包含「答题用户端」和「答题管理端」。

不要做复杂的题库系统、培训系统、赛事系统，只专注于「一场线上答题活动」本身。

核心流程：

**创建答题 → 用户进入 → 发布题目 → 用户答题 → 自动判分 → 抢答 → 实时排名 → 答题结束**

---

# 一、技术要求

推荐技术栈：

前端：

* Vue 
* TypeScript
* Vite

后端：

* Go

数据库：

* MySQL

实时通信：

* WebSocket

缓存：

* Redis

部署：

* Docker Compose

---

# 二、系统角色

只需要两个角色：

## 管理员

负责：

* 创建答题
* 添加题目
* 设置答题规则
* 控制开始/暂停/结束
* 发布题目
* 开始抢答
* 查看实时答题情况
* 查看排行榜

## 答题用户

负责：

* 输入姓名/昵称
* 进入答题
* 查看题目
* 提交答案
* 参与抢答
* 查看自己的成绩
* 查看最终排名

不需要复杂的用户注册系统。

---

# 三、答题管理端

## 1. 创建答题

管理员创建一场答题。

配置：

* 答题名称
* 答题说明
* 答题模式
* 是否开启抢答
* 每题答题时间
* 总答题时间
* 是否显示正确答案
* 是否显示解析
* 是否显示排行榜

创建完成后生成：

```text
答题ID
答题链接
答题邀请码
```

---

# 四、添加题目

支持三种题型：

## 单选题

只能选择一个答案。

例如：

> HTTP 默认端口是多少？

A. 21

B. 80

C. 443

D. 3306

---

## 多选题

可以选择多个答案。

例如：

> 以下哪些属于 Web 漏洞？

A. SQL Injection

B. XSS

C. SSRF

D. FTP

只有完全匹配正确答案才算正确。

---

## 判断题

选项：

* 正确
* 错误

---

# 五、题目配置

每道题支持：

* 题目内容
* 题型
* 选项
* 正确答案
* 解析
* 分值
* 是否必答
* 答题时间

例如：

```text
题目：HTTP 默认端口是多少？

类型：单选

分值：10

必答：是

答题时间：20秒

正确答案：B

解析：HTTP 默认使用 TCP 80 端口。
```

---

# 六、必答

每道题可以设置：

```text
必答：是
必答：否
```

必答题：

用户必须提交答案后才能进入下一题。

非必答题：

允许跳过。

---

# 七、用户进入答题

用户打开答题链接：

```text
输入昵称
↓
输入答题邀请码
↓
进入答题
```

进入后显示：

* 答题名称
* 答题规则
* 当前参与人数
* 开始状态

管理员没有开始答题之前：

```text
等待管理员开始...
```

管理员开始后自动进入第一题。

---

# 八、答题页面

答题页面必须简洁。

顶部：

```text
答题名称
第 3 / 20 题
当前得分
倒计时
```

中间：

```text
题目

○ A. xxx

○ B. xxx

○ C. xxx

○ D. xxx
```

底部：

```text
上一题
下一题
提交答案
```

移动端也必须正常使用。

---

# 九、倒计时

支持每题倒计时。

例如：

```text
20
19
18
17
...
3
2
1
0
```

时间到：

自动提交。

如果用户没有选择：

* 必答题：记录未答
* 非必答题：直接进入下一题

倒计时必须以服务器时间为准。

不能完全依赖浏览器 JavaScript 的本地倒计时。

---

# 十、普通答题模式

流程：

```text
管理员开始
↓
发布第 1 题
↓
用户看到第 1 题
↓
用户选择答案
↓
提交
↓
服务器判分
↓
进入下一题
↓
全部完成
↓
显示成绩
```

管理员可以控制：

```text
开始
暂停
继续
下一题
上一题
结束
```

---

# 十一、抢答模式

抢答是系统的核心功能之一。

流程：

```text
管理员发布题目
↓
所有用户看到题目
↓
管理员点击「开始抢答」
↓
用户看到「立即抢答」
↓
用户点击
↓
服务器记录抢答时间
↓
确定抢答排名
↓
抢答成功用户获得答题资格
↓
进行答题
↓
自动判分
↓
更新排行榜
```

---

# 十二、抢答按钮

未开始：

```text
等待抢答
```

开始后：

```text
🔥 立即抢答
```

抢答成功：

```text
🎉 抢答成功
你获得本题答题资格
```

抢答失败：

```text
很遗憾
本题抢答资格已被其他用户获得
```

---

# 十三、抢答并发

必须保证抢答结果由服务器决定。

例如 100 个用户同时点击：

```text
User A
User B
User C
...
User 100
```

服务器按照收到请求的服务器时间决定顺序。

不能使用客户端时间。

必须使用 Redis 原子操作，例如：

```text
SETNX
```

或者 Lua Script。

必须保证：

* 第一名只能有一个
* 不允许重复抢答
* 不允许重复得分
* 不允许伪造时间
* 并发情况下结果稳定

---

# 十四、抢答规则

管理员可以设置：

```text
每题抢答人数：1
抢答时间：10秒
抢答成功后答题时间：20秒
```

也支持：

```text
第一名：10分
第二名：5分
第三名：3分
```

如果只允许一个人抢答，则其他用户直接显示：

```text
本题抢答结束
```

---

# 十五、积分规则

普通答题：

```text
答对：获得题目分值
答错：0分
```

抢答：

可以配置：

```text
抢答奖励分
答题分
答错扣分
```

例如：

```text
抢答成功：+5
答对：+10
答错：-5
```

最终：

```text
总分 = 抢答分 + 答题分 - 扣分
```

所有分数必须由后端计算。

---

# 十六、实时排行榜

答题过程中实时显示排行榜。

例如：

```text
🏆 实时排行榜

1  张三     85分
2  李四     80分
3  王五     75分
4  赵六     70分
```

排行榜通过 WebSocket 实时更新。

不需要刷新页面。

支持：

* 排名
* 昵称
* 分数
* 正确题数

---

# 十七、管理员控制台

管理员页面只做答题控制。

页面分成三部分。

## 左侧

题目列表：

```text
01 单选
02 判断
03 多选
04 单选
05 判断
```

当前题目高亮。

## 中间

当前题目：

```text
第 3 题

以下哪些属于 Web 漏洞？

A. SQL Injection
B. XSS
C. SSRF
D. FTP
```

## 右侧

实时数据：

```text
参与人数：156

已答：132

未答：24

正确：108

错误：24

当前最高分：80
```

底部按钮：

```text
上一题
开始答题
开始抢答
暂停
公布答案
下一题
结束答题
```

---

# 十八、管理员实时看到答题情况

当前题目实时显示：

```text
156 人参与

A：32人
B：87人
C：21人
D：16人
```

多选题显示：

```text
AB：20
AC：10
BC：15
ABC：50
```

判断题：

```text
正确：120
错误：36
```

这些数据通过 WebSocket 实时更新。

---

# 十九、公布答案

管理员点击：

```text
公布答案
```

所有用户实时看到：

```text
正确答案：B

你的答案：B

回答正确！

+10 分
```

如果答错：

```text
正确答案：B

你的答案：C

回答错误

+0 分
```

如果管理员关闭「显示答案」，则用户看不到正确答案。

---

# 二十、答题结束

结束后用户看到：

```text
🎉 答题完成

总分：85

答题数量：20

正确：17

错误：3

正确率：85%

用时：08:32

当前排名：12
```

如果开启排行榜：

显示：

```text
最终排行榜
```

---

# 二十一、管理员统计

活动结束后管理员可以看到：

```text
参与人数
完成数量
平均分
最高分
最低分
平均正确率
```

题目统计：

```text
题目
答题人数
正确人数
错误人数
正确率
平均答题时间
```

例如：

```text
第 1 题：正确率 92%
第 2 题：正确率 75%
第 3 题：正确率 38%
```

方便管理员发现难题。

---

# 二十二、WebSocket

使用 WebSocket 实现实时同步。

至少包含：

```text
activity:start
activity:pause
activity:end

question:publish
question:next
question:previous

question:countdown

answer:submit
answer:result

rush:start
rush:submit
rush:success
rush:failed
rush:end

ranking:update

statistics:update

answer:reveal
```

---

# 二十三、服务器状态

答题状态必须由服务器维护。

例如：

```text
WAITING
RUNNING
PAUSED
RUSHING
ANSWERING
REVEALING
FINISHED
```

不能让客户端自行决定：

* 当前题目
* 答题是否结束
* 抢答是否开始
* 是否抢答成功
* 得分
* 排名

客户端只负责展示。

---

# 二十四、数据表

只保留答题系统需要的数据。

```text
users
```

用户：

```text
id
nickname
created_at
```

---

```text
quizzes
```

答题：

```text
id
title
description
status
mode
total_time
created_at
started_at
ended_at
```

---

```text
questions
```

题目：

```text
id
quiz_id
type
content
answer
analysis
score
required
sort
time_limit
```

---

```text
question_options
```

选项：

```text
id
question_id
label
content
sort
```

---

```text
participants
```

参与者：

```text
id
quiz_id
user_id
score
correct_count
wrong_count
joined_at
```

---

```text
answers
```

答题记录：

```text
id
quiz_id
question_id
user_id
answer
is_correct
score
duration
submitted_at
```

---

```text
rush_records
```

抢答：

```text
id
quiz_id
question_id
user_id
server_time
rank
score
created_at
```

---

# 二十五、API

用户端：

```text
POST /api/join

GET /api/quiz/:id

GET /api/quiz/:id/current-question

POST /api/question/:id/answer

POST /api/question/:id/rush

GET /api/quiz/:id/ranking

GET /api/quiz/:id/result
```

管理员：

```text
POST /api/admin/quiz

PUT /api/admin/quiz/:id

POST /api/admin/quiz/:id/start

POST /api/admin/quiz/:id/pause

POST /api/admin/quiz/:id/next

POST /api/admin/quiz/:id/previous

POST /api/admin/quiz/:id/rush/start

POST /api/admin/quiz/:id/rush/end

POST /api/admin/quiz/:id/reveal

POST /api/admin/quiz/:id/end

GET /api/admin/quiz/:id/statistics
```

---

# 二十六、UI设计

整体风格：

* 简洁
* 现代
* 科技感
* 大按钮
* 强视觉反馈
* 适合比赛/课堂现场

用户端重点突出：

```text
题目
选项
倒计时
提交
抢答
积分
```

抢答按钮：

必须足够大，在手机上也方便点击。

颜色状态：

```text
等待：灰色
可抢答：强调色
抢答成功：绿色
抢答失败：红色
正确：绿色
错误：红色
```

---

# 二十七、断线重连

WebSocket 必须支持：

* 心跳
* 自动重连
* 重连后恢复状态

用户刷新浏览器后：

* 自动恢复当前答题
* 恢复当前题目
* 恢复答题状态
* 恢复积分

不能因为刷新页面导致答题记录丢失。

---

# 二十八、安全要求

必须做到：

* JWT / Session 鉴权
* WebSocket 鉴权
* 用户只能提交自己的答案
* 后端判题
* 后端计算分数
* 防重复提交
* 防重复计分
* 抢答服务端判断
* 正确答案不能提前发送给客户端

---

# 二十九、最终必须实现的完整流程

管理员：

```text
创建答题
↓
添加题目
↓
设置题目分值
↓
设置必答
↓
设置答题时间
↓
发布答题
↓
等待用户进入
↓
开始答题
↓
发布第 1 题
↓
用户答题
↓
自动判分
↓
下一题
↓
开始抢答
↓
用户抢答
↓
服务器确定抢答结果
↓
抢答者答题
↓
自动判分
↓
实时排行榜
↓
公布答案
↓
下一题
↓
结束答题
↓
生成最终成绩和排行榜
```

用户：

```text
输入昵称
↓
进入答题
↓
等待开始
↓
看到题目
↓
选择答案
↓
提交
↓
获得分数
↓
参加抢答
↓
查看实时排名
↓
完成所有题目
↓
查看最终成绩
```

---

# 三十、开发要求

不要只生成页面原型。

必须真正实现：

**前端 + 后端 + 数据库 + Redis + WebSocket + 答题逻辑 + 抢答逻辑 + 自动判分 + 实时排行榜。**

优先完成最核心的 MVP：

> **管理员创建答题 → 用户进入 → 管理员发布题目 → 用户答题 → 自动判分 → 抢答 → 实时排名 → 结束答题**

所有核心按钮必须真实可用，不允许使用 Mock 数据代替核心业务逻辑。

