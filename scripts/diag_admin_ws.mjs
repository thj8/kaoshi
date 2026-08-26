// 诊断：管理端 WS 是否实时收到 rush/statistics/reveal 事件
import { readFileSync } from 'node:fs'
const env = k => process.env[k] || (readFileSync('.env', 'utf8').match(new RegExp('^' + k + '=(.*)$', 'm')) || [])[1] || ''
const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const j = async (m, u, b, tok) => {
  const r = await fetch(B + u, { method: m, headers: { 'Content-Type': 'application/json', ...(tok ? { Authorization: 'Bearer ' + tok } : {}) }, body: b ? JSON.stringify(b) : undefined })
  return r.json()
}
const sleep = ms => new Promise(r => setTimeout(r, ms))

function connect(url, token, onMsg) {
  return new Promise((res, rej) => {
    const ws = new WebSocket(url, [token])
    const t = setTimeout(() => rej(new Error('ws timeout')), 8000)
    ws.onmessage = e => { try { onMsg?.(JSON.parse(e.data)) } catch {} }
    ws.onopen = () => { clearTimeout(t); res(ws) }
    ws.onerror = () => { clearTimeout(t); rej(new Error('ws error')) }
  })
}

const at = (await j('POST', '/api/admin/login', { username: 'admin', password: env('ADMIN_PASS') })).data.token
const sfx = Date.now() % 100000
const quiz = (await j('POST', '/api/admin/quiz', { title: `diag-rush-${sfx}`, mode: 'rush', rush_winner_count: 1, rush_time: 10, rush_answer_time: 20, rush_bonus_score: 5, show_answer: true, show_ranking: true }, at)).data
const q = (await j('POST', `/api/admin/quiz/${quiz.id}/questions`, { type: 'single', content: 'diag?', answer: 'B', score: 10, required: false, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
await j('POST', '/api/admin/users', { username: `dg${sfx}`, password: 'test-pass-1234', nickname: '诊断员' }, at)
const u = (await j('POST', '/api/auth/login', { username: `dg${sfx}`, password: 'test-pass-1234' })).data
const jt = (await j('POST', '/api/join', { quiz_id: quiz.id }, u.token)).data.token

// 管理端 WS（与 ConsolePage 相同：?quiz= + admin token）
const adminGot = []
const aws = await connect(`${B.replace(/^http/, 'ws')}/ws?quiz=${quiz.id}`, at, m => {
  if (m.event !== 'question:countdown') adminGot.push(m.event + ' ' + JSON.stringify(m.data).slice(0, 160))
})
console.log('admin ws connected')

await j('POST', `/api/admin/quiz/${quiz.id}/start`, {}, at); await sleep(500)
await j('POST', `/api/admin/quiz/${quiz.id}/rush/start`, {}, at); await sleep(500)
const rush = await j('POST', `/api/question/${q.id}/rush`, {}, jt)
console.log('user rush resp:', JSON.stringify(rush))
await sleep(600)
// rush/end（engine 可能自动 end；若 status 仍 RUSHING 则手动）
const st = await j('GET', `/api/admin/quiz/${quiz.id}`, null, at)
if (st.data.status === 'RUSHING') await j('POST', `/api/admin/quiz/${quiz.id}/rush/end`, {}, at)
await sleep(500)
const ans = await j('POST', `/api/question/${q.id}/answer`, { answer: 'A', duration: 500 }, jt)
console.log('user answer resp:', JSON.stringify(ans))
await sleep(500)
await j('POST', `/api/admin/quiz/${quiz.id}/reveal`, {}, at)
await sleep(800)

console.log('--- admin WS received ---')
for (const l of adminGot) console.log(' ', l)
aws.close(); process.exit(0)
