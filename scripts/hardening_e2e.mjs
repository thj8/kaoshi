// 阶段8 加固 E2E：鉴权越权 / 防重复计分 / 答案泄露 / 100并发抢答 / 断线重连恢复
import { readFileSync } from 'node:fs'
const env = k => process.env[k] || (readFileSync('.env', 'utf8').match(new RegExp('^' + k + '=(.*)$', 'm')) || [])[1] || ''
const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const j = async (m, u, b, tok) => {
  const r = await fetch(B + u, { method: m, headers: { 'Content-Type': 'application/json', ...(tok ? { Authorization: 'Bearer ' + tok } : {}) }, body: b ? JSON.stringify(b) : undefined })
  return r.json()
}
const sleep = ms => new Promise(r => setTimeout(r, ms))
let pass = 0, fail = 0
const check = (name, ok, extra = '') => { ok ? pass++ : fail++; console.log(`${ok ? '✅' : '❌'} ${name}${extra ? '  ' + extra : ''}`) }

function connect(token, onMsg) {
  return new Promise((res, rej) => {
    const ws = new WebSocket(`${B.replace(/^http/, "ws")}/ws`, [token])
    const t = setTimeout(() => rej(new Error('ws timeout')), 8000)
    ws.onmessage = e => { try { onMsg?.(JSON.parse(e.data)) } catch {} }
    ws.onopen = () => { clearTimeout(t); res(ws) }
    ws.onerror = () => { clearTimeout(t); rej(new Error('ws error')) }
  })
}

;(async () => {
  const at = (await j('POST', '/api/admin/login', { username: 'admin', password: env('ADMIN_PASS') })).data.token
  const mkU = async u => { // 注册已下线：admin 建号 + 登录
    const NAMES = ['陈晨', '刘洋', '赵磊', '孙悦', '周杰', '吴倩', '郑浩', '王梅']
    await j('POST', '/api/admin/users', { username: u, password: 'test-pass-1234', nickname: NAMES[uN++ % NAMES.length] }, at)
    return (await j('POST', '/api/auth/login', { username: u, password: 'test-pass-1234' })).data
  }
  let uN = 0
  const sfx = Date.now() % 100000

  // ---------- 1. 普通 quiz：鉴权 / 防重复 / 答案泄露 ----------
  const quizA = (await j('POST', '/api/admin/quiz', { title: 's8-secA', mode: 'normal', per_question_time: 60, show_answer: true }, at)).data
  const quizB = (await j('POST', '/api/admin/quiz', { title: 's8-secB', mode: 'normal', per_question_time: 60 }, at)).data
  const qA = (await j('POST', `/api/admin/quiz/${quizA.id}/questions`, { type: 'single', content: 'A?', answer: 'B', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  await j('POST', `/api/admin/quiz/${quizB.id}/questions`, { type: 'single', content: 'B?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)

  const ua = await mkU(`s8u${sfx}`)
  const ja = (await j('POST', '/api/join', { quiz_id: quizA.id }, ua.token)).data
  const jb = (await j('POST', '/api/join', { quiz_id: quizB.id }, ua.token)).data

  // 1a. quizB 的 token 不能操作 quizA 的题
  await j('POST', `/api/admin/quiz/${quizA.id}/start`, {}, at); await sleep(400)
  const cross = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, jb.token)
  check('越权：跨 quiz token 提交被拒', cross.code !== 0, `code=${cross.code} msg=${cross.msg}`)

  // 1b. 未参加 quizA 的裸 token 也不能
  const stranger = await mkU(`s8x${sfx}`)
  const noJoin = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, stranger.token)
  check('越权：未参加者提交被拒', noJoin.code !== 0, `code=${noJoin.code}`)

  // 1c. 重复提交只计一次分
  const r1 = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, ja.token)
  const r2 = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, ja.token)
  const res1 = (await j('GET', `/api/quiz/${quizA.id}/result`, null, ja.token)).data
  check('防重复：二次提交被拒或不再加分', res1.score === 10, `first.code=${r1.code} second.code=${r2.code} score=${res1.score}`)

  // 1d. 答案不下发：题目相关公开接口均无 answer/analysis 字段
  const cur = JSON.stringify(await j('GET', `/api/quiz/${quizA.id}/current-question`, null, ja.token))
  const info = JSON.stringify(await j('GET', `/api/quiz/${quizA.id}`, null, ja.token))
  check('答案不下发：current-question/info 无 answer/analysis', !/"answer":"[AB]"|"analysis":"/.test(cur + info), '')
  await j('POST', `/api/admin/quiz/${quizA.id}/end`, {}, at)

  // ---------- 2. 100 并发抢答唯一性 ----------
  const rq = (await j('POST', '/api/admin/quiz', { title: 's8-rush100', mode: 'rush', rush_winner_count: 1, rush_time: 15, rush_answer_time: 20, rush_bonus_score: 5, show_ranking: true }, at)).data
  const qr = (await j('POST', `/api/admin/quiz/${rq.id}/questions`, { type: 'single', content: 'R?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const N = 100
  const tokens = []
  for (let i = 0; i < N; i++) {
    const u = await mkU(`s8r${sfx}_${i}`)
    tokens.push((await j('POST', '/api/join', { quiz_id: rq.id }, u.token)).data.token)
  }
  await j('POST', `/api/admin/quiz/${rq.id}/start`, {}, at)
  await j('POST', `/api/admin/quiz/${rq.id}/rush/start`, {}, at)
  await sleep(400)
  const results = await Promise.allSettled(tokens.map(t => j('POST', `/api/question/${qr.id}/rush`, {}, t)))
  const winners = results.filter(r => r.status === 'fulfilled' && r.value.code === 0 && r.value.data?.rank === 1)
  const losers = results.filter(r => r.status === 'fulfilled' && r.value.code === 0 && r.value.data?.rank > 1)
  check(`100并发抢答：rank=1 唯一`, winners.length === 1, `winners=${winners.length} losers=${losers.length} errors=${results.length - winners.length - losers.length}`)
  await j('POST', `/api/admin/quiz/${rq.id}/rush/end`, {}, at)

  // ---------- 2b. 强制收卷后实时统计：未答占位行不计入已答/正确/错误 ----------
  const quizS = (await j('POST', '/api/admin/quiz', { title: 's8-stats', mode: 'normal', per_question_time: 60, show_answer: true }, at)).data
  const qS = (await j('POST', `/api/admin/quiz/${quizS.id}/questions`, { type: 'single', content: 'S?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const us1 = await mkU(`s8s1${sfx}`), us2 = await mkU(`s8s2${sfx}`), us3 = await mkU(`s8s3${sfx}`)
  const js1 = (await j('POST', '/api/join', { quiz_id: quizS.id }, us1.token)).data
  const js2 = (await j('POST', '/api/join', { quiz_id: quizS.id }, us2.token)).data
  const js3 = (await j('POST', '/api/join', { quiz_id: quizS.id }, us3.token)).data
  await j('POST', `/api/admin/quiz/${quizS.id}/start`, {}, at); await sleep(300)
  await j('POST', `/api/question/${qS.id}/answer`, { answer: 'A', duration: 100 }, js1.token) // 1 人答对
  await j('POST', `/api/question/${qS.id}/answer`, { answer: 'B', duration: 100 }, js2.token) // 1 人答错
  // us3 超时不答 → 强制收卷补未答占位行
  await j("POST", `/api/admin/quiz/${quizS.id}/reveal`, {}, at); await sleep(300)
  const stS = (await j('GET', `/api/admin/quiz/${quizS.id}/statistics`, null, at)).data
  const qSt = stS.questions?.find(x => x.question_id === qS.id)
  check('实时统计：已答不含未答占位(2/3)', qSt && qSt.answered === 2, `answered=${qSt?.answered}`)
  check('实时统计：正确/错误不含未答(1/1)', qSt && qSt.correct === 1 && qSt.wrong === 1, `correct=${qSt?.correct} wrong=${qSt?.wrong}`)
  check('实时统计：正确率分母为真实作答(50%)', Math.round(qSt.correct_rate) === 50, `rate=${qSt?.correct_rate}`)

  // ---------- 3. 断线重连恢复 ----------
  const quizC = (await j('POST', '/api/admin/quiz', { title: 's8-reconnect', mode: 'normal', per_question_time: 120 }, at)).data
  const qC = (await j('POST', `/api/admin/quiz/${quizC.id}/questions`, { type: 'single', content: 'C?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const uc = await mkU(`s8c${sfx}`)
  const jc = (await j('POST', '/api/join', { quiz_id: quizC.id }, uc.token)).data
  let synced = null
  const ws1 = await connect(jc.token, m => { if (m.event === 'sync') synced = m.data })
  await j('POST', `/api/admin/quiz/${quizC.id}/start`, {}, at); await sleep(500)
  ws1.close() // 断线
  await sleep(300)
  const ws2 = await connect(jc.token, m => { if (m.event === 'sync') synced = m.data })
  await sleep(500)
  check('重连恢复：sync 带回状态与当前题', !!synced && synced.status === 'ANSWERING' && synced.question?.id === qC.id, `status=${synced?.status} q=${synced?.question?.id}`)
  const sub = await j('POST', `/api/question/${qC.id}/answer`, { answer: 'A', duration: 300 }, jc.token)
  check('重连后仍可作答', sub.code === 0, `code=${sub.code}`)
  ws2.close()
  await j('POST', `/api/admin/quiz/${quizC.id}/end`, {}, at)

  // 清理
  for (const q of [quizA.id, quizB.id, rq.id, quizC.id]) await j('DELETE', `/api/admin/quiz/${q}`, null, at)
  console.log(`\n${fail === 0 ? 'ALL PASS' : 'HAS FAILURES'}: ${pass} passed, ${fail} failed`)
  process.exit(fail === 0 ? 0 : 1)
})().catch(e => { console.error('FAIL:', e.message); process.exit(1) })
