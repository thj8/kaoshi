// 构建模拟测试数据：真实姓名用户 + 一场进行中的答题活动（含部分作答成绩）
// 用法：node scripts/seed_test_data.mjs [场次前缀]   （默认 demo，可重复跑，自动加时间戳后缀）
import { readFileSync } from 'node:fs'
const env = k => process.env[k] || (readFileSync('.env', 'utf8').match(new RegExp('^' + k + '=(.*)$', 'm')) || [])[1] || ''
const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const j = async (m, u, b, tok) => {
  const r = await fetch(B + u, { method: m, headers: { 'Content-Type': 'application/json', ...(tok ? { Authorization: 'Bearer ' + tok } : {}) }, body: b ? JSON.stringify(b) : undefined })
  const d = await r.json()
  if (d.code !== 0) throw new Error(`${m} ${u} -> ${d.msg}`)
  return d.data
}

const USERS = [
  { username: 'zhangwei', nickname: '张伟' },
  { username: 'lina', nickname: '李娜' },
  { username: 'wangfang', nickname: '王芳' },
  { username: 'liuyang', nickname: '刘洋' },
  { username: 'chenjing', nickname: '陈静' },
]
const PASSWORD = 'test12345678'

const QS = [
  { type: 'single', content: '《安全生产法》规定，生产经营单位的主要负责人是本单位安全生产第一责任人。', answer: 'A', analysis: '新安法第五条明确。', score: 10, required: true, time_limit: 30, options: [{ label: 'A', content: '正确' }, { label: 'B', content: '错误' }] },
  { type: 'single', content: '火灾报警电话是以下哪一个？', answer: 'C', analysis: '火警 119，急救 120，报警 110。', score: 10, required: true, time_limit: 30, options: [{ label: 'A', content: '110' }, { label: 'B', content: '120' }, { label: 'C', content: '119' }, { label: 'D', content: '122' }] },
  { type: 'multiple', content: '下列哪些属于劳动防护用品？（多选）', answer: 'AC', analysis: '安全帽与绝缘手套属于劳动防护用品。', score: 10, required: true, time_limit: 40, options: [{ label: 'A', content: '安全帽' }, { label: 'B', content: '办公椅' }, { label: 'C', content: '绝缘手套' }, { label: 'D', content: '饮水机' }] },
  { type: 'judge', content: '发现燃气泄漏时可以立即开灯查看。', answer: 'B', analysis: '应先关阀开窗，杜绝明火与电火花。', score: 10, required: true, time_limit: 20, options: [{ label: 'A', content: '正确' }, { label: 'B', content: '错误' }] },
  { type: 'single', content: '灭火器使用时的“提、拔、握、压”四步中，“拔”是指？', answer: 'B', analysis: '拔掉保险销。', score: 10, required: true, time_limit: 30, options: [{ label: 'A', content: '拔掉软管' }, { label: 'B', content: '拔掉保险销' }, { label: 'C', content: '拔掉喷嘴' }, { label: 'D', content: '拔掉压力表' }] },
]

// 每个用户的前几题作答（answer 为空串 = 该题不答）
const PLANS = [
  { nickname: '张伟', answers: ['A', 'C', 'AC', 'B', 'B'] },
  { nickname: '李娜', answers: ['A', 'C', 'AC', 'B', 'A'] },
  { nickname: '王芳', answers: ['A', 'C', 'AB', 'B', 'B'] },
  { nickname: '刘洋', answers: ['B', 'C', 'AC', 'B', ''] },
  { nickname: '陈静', answers: ['A', 'B', 'AC', '', ''] },
]

;(async () => {
  const at = (await j('POST', '/api/admin/login', { username: 'admin', password: env('ADMIN_PASS') })).token
  const sfx = Date.now() % 100000

  // 1. 用户（已存在则跳过）
  const existing = await j('GET', '/api/admin/users', null, at)
  const have = new Set(existing.map(u => u.username))
  for (const u of USERS) {
    if (have.has(u.username)) { console.log('· 用户已存在:', u.nickname); continue }
    await j('POST', '/api/admin/users', { username: u.username, password: PASSWORD, nickname: u.nickname }, at)
    console.log('＋ 用户:', u.nickname, u.username)
  }

  // 2. 答题活动
  const quiz = (await j('POST', '/api/admin/quiz', {
    title: `安全知识理论竞赛（演示${sfx}）`,
    description: '安全生产与消防常识，共 5 题，每人 10 分/题',
    mode: 'normal', per_question_time: 30, total_time: 0,
    show_answer: true, show_analysis: true, show_ranking: true,
  }, at))
  for (const q of QS) await j('POST', `/api/admin/quiz/${quiz.code}/questions`, q, at)
  console.log('＋ 活动: 安全知识理论竞赛（演示' + sfx + '） #' + quiz.code)

  // 3. 加入 + 作答（走完整业务流，产生真实成绩/排行榜）
  await j('POST', `/api/admin/quiz/${quiz.code}/start`, null, at)
  const qList = await j('GET', `/api/admin/quiz/${quiz.code}/questions`, null, at)
  // 所有用户加入
  const tokens = []
  for (const plan of PLANS) {
    const u = USERS.find(x => x.nickname === plan.nickname)
    const login = (await j('POST', '/api/auth/login', { username: u.username, password: PASSWORD })).token
    const { token } = await j('POST', '/api/join', { quiz_id: quiz.code }, login)
    tokens.push({ nickname: plan.nickname, token })
  }
  // 按题推进：全员作答 → 公布 → 下一题
  for (let i = 0; i < qList.length; i++) {
    for (const t of tokens) {
      const ans = PLANS.find(p => p.nickname === t.nickname).answers[i]
      if (!ans) continue
      await j('POST', `/api/question/${qList[i].id}/answer`, { answer: ans, duration: 3000 + i * 1500 }, t.token)
    }
    await j('POST', `/api/admin/quiz/${quiz.code}/reveal`, null, at).catch(() => {})
    await j('POST', `/api/admin/quiz/${quiz.code}/next`, null, at).catch(() => {})
    console.log('＋ 第', i + 1, '题作答完成')
  }
  await j('POST', `/api/admin/quiz/${quiz.code}/end`, null, at).catch(() => {})
  console.log('✅ 种子数据完成：5 名用户 · 1 场已完结竞赛（含成绩与排行榜）')
})().catch(e => { console.error('❌', e.message); process.exit(1) })
