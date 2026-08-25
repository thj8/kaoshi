// 生成演示/验证用测试数据：清库 → 建 3 个用户 + 2 场答题（普通模式、抢答模式）
// 用法：node scripts/seed.mjs
import { execSync as sh } from 'node:child_process'

const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const ADMIN_PASS = process.env.ADMIN_PASS || '***REMOVED***'
const MYSQL_PASS = process.env.MYSQL_PASS || '***REMOVED***'
const REDIS_PASS = process.env.REDIS_PASS || '***REMOVED***'

const j = async (method, path, body, token) => {
  const r = await fetch(B + path, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
    body: body === null ? undefined : JSON.stringify(body),
  })
  return r.json()
}

// ---------- 1. 清空所有数据 ----------
sh(`docker exec kaoshi-mysql mysql -uroot -p${MYSQL_PASS} kaoshi -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"`, { stdio: ['ignore', 'pipe', 'pipe'] })
sh(`docker exec kaoshi-redis redis-cli -a ${REDIS_PASS} FLUSHDB`, { stdio: ['ignore', 'pipe', 'pipe'] })
try { sh('docker restart kaoshi-server', { stdio: ['ignore', 'pipe', 'pipe'] }) } catch {}
console.log('🗑  已清空全部数据（MySQL + Redis + server Runtime）')

let at = null
for (let i = 0; i < 30 && !at; i++) {
  try { at = (await j('POST', '/api/admin/login', { username: 'admin', password: ADMIN_PASS })).data.token } catch { await new Promise(r => setTimeout(r, 1000)) }
}
if (!at) throw new Error('admin 登录失败（server 未就绪？）')

// ---------- 2. 用户账号 ----------
const USERS = [
  { username: 'player1', password: 'pass1234', nickname: '选手一号' },
  { username: 'player2', password: 'pass1234', nickname: '选手二号' },
  { username: 'player3', password: 'pass1234', nickname: '选手三号' },
]
for (const u of USERS) {
  const r = await j('POST', '/api/auth/register', u)
  if (r.code !== 0) throw new Error('注册失败 ' + u.username + ': ' + r.msg)
}

// ---------- 3. 答题活动 ----------
const opts = (...labels) => labels.map((l, i) => ({ label: l, content: '选项 ' + l }))
const mkQuiz = (title, mode, extra = {}) => j('POST', '/api/admin/quiz', {
  title, mode, per_question_time: 30, rush_time: 10, rush_answer_time: 15, rush_bonus_score: 5,
  show_answer: true, show_analysis: true, show_ranking: true, ...extra,
}, at)
const mkQ = (quizID, q) => j('POST', `/api/admin/quiz/${quizID}/questions`, q, at)

// 3.1 普通答题：常识问答（5 题：4 单选 + 1 多选）
const q1 = (await mkQuiz('常识知识竞赛', 'normal')).data
await mkQ(q1.id, { type: 'single', content: '中国的首都是哪座城市？', answer: 'B', analysis: '北京是中华人民共和国首都。', score: 10, required: true, time_limit: 20, sort: 1, options: opts('A', 'B', 'C', 'D') })
await mkQ(q1.id, { type: 'single', content: '一年有多少天（非闰年）？', answer: 'C', analysis: '非闰年 365 天。', score: 10, required: true, time_limit: 20, sort: 2, options: opts('A', 'B', 'C', 'D') })
await mkQ(q1.id, { type: 'single', content: '光速大约是每秒多少公里？', answer: 'A', analysis: '约 30 万 km/s。', score: 10, required: true, time_limit: 20, sort: 3, options: opts('A', 'B', 'C', 'D') })
await mkQ(q1.id, { type: 'single', content: '《静夜思》的作者是谁？', answer: 'D', analysis: '李白。', score: 10, required: true, time_limit: 20, sort: 4, options: opts('A', 'B', 'C', 'D') })
await mkQ(q1.id, { type: 'multiple', content: '以下哪些是编程语言？（多选）', answer: 'ACD', analysis: 'Python/C++/Go 是编程语言，HTTP 是协议。', score: 20, required: true, time_limit: 30, sort: 5, options: opts('A', 'B', 'C', 'D') })

// 3.2 抢答模式：极速抢答赛（3 题）
const q2 = (await mkQuiz('极速抢答赛', 'rush')).data
await mkQ(q2.id, { type: 'single', content: '1+1 = ?', answer: 'B', analysis: '基础加法。', score: 10, required: true, time_limit: 15, sort: 1, options: opts('A', 'B', 'C', 'D') })
await mkQ(q2.id, { type: 'single', content: '彩虹有几种颜色？', answer: 'C', analysis: '红橙黄绿蓝靛紫七种。', score: 10, required: true, time_limit: 15, sort: 2, options: opts('A', 'B', 'C', 'D') })
await mkQ(q2.id, { type: 'single', content: '汉字「一」有几画？', answer: 'A', analysis: '一画。', score: 10, required: true, time_limit: 15, sort: 3, options: opts('A', 'B', 'C', 'D') })

console.log(`✅ 测试数据已生成：

📋 管理员：admin / ${ADMIN_PASS}   →  http://<IP>:13000/admin/login

👥 选手账号（/login 登录后经 /join/<quizID> 参赛）：
   player1 / pass1234（选手一号）
   player2 / pass1234（选手二号）
   player3 / pass1234（选手三号）

🎯 答题活动（均为 WAITING 待开始，管理端点「开始」即可开赛）：
   #1 常识知识竞赛（普通模式，5 题，每题 20-30s，reveal 显示答案+解析）
   #2 极速抢答赛（抢答模式，3 题，抢答窗 10s，答题 15s）`)
process.exit(0)
