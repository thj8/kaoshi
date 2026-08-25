// 理论答题安全 E2E：抢答权限 / 答案回显 / reveal 门控 / 状态机 / 越权 / 倒计时
// 用法：node scripts/security_e2e.mjs   （NO_CLEAN=1 跳过开头清库）
// 开头清空 MySQL 各表 + Redis 抢答状态，再建测试用户与题目；任一断言失败退出码 1
import { execSync as sh } from 'node:child_process'
const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const MYSQL_PASS = process.env.MYSQL_PASS || '***REMOVED***'
const REDIS_PASS = process.env.REDIS_PASS || '***REMOVED***'
const ADMIN_PASS = process.env.ADMIN_PASS || '***REMOVED***'

const j = async (m, u, b, tok) => {
  const r = await fetch(B + u, { method: m, headers: { 'Content-Type': 'application/json', ...(tok ? { Authorization: 'Bearer ' + tok } : {}) }, body: b ? JSON.stringify(b) : undefined })
  return r.json()
}
const sleep = ms => new Promise(r => setTimeout(r, ms))
let pass = 0, fail = 0
const check = (name, ok, extra = '') => { ok ? pass++ : fail++; console.log(`${ok ? '✅' : '❌'} ${name}${extra ? '  ' + extra : ''}`) }
const leakSecret = s => /"correct_answer"\s*:\s*"[^"]+"/.test(s) || /"analysis"\s*:\s*"[^"]+"/.test(s)
const leak = s => leakSecret(s) || /"answer"\s*:\s*"[A-D]"/.test(s)

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
  // ---------- 0. 清理数据库 + Redis（幂等，失败则时间戳后缀隔离兜底） ----------
  if (process.env.NO_CLEAN !== '1') {
    try {
      sh(`docker exec kaoshi-mysql mysql -uroot -p${MYSQL_PASS} kaoshi -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"`, { stdio: ['ignore', 'pipe', 'pipe'] })
      sh(`docker exec kaoshi-redis redis-cli -a ${REDIS_PASS} FLUSHDB`, { stdio: ['ignore', 'pipe', 'pipe'] })
      // 自增 ID 复用会撞上 server 内存里的旧 Runtime，重启清空（本机容器场景）
      try { sh('docker restart kaoshi-server', { stdio: ['ignore', 'pipe', 'pipe'] }) } catch {}
      console.log('🗑  已清库（MySQL 各表 + Redis 抢答状态 + server Runtime）')
    } catch (e) {
      console.log('⚠ 清理失败（本机无容器？）', String(e).split('\n')[0])
    }
  }

  let at = null
  for (let i = 0; i < 30 && !at; i++) {
    try { at = (await j('POST', '/api/admin/login', { username: 'admin', password: ADMIN_PASS })).data.token } catch { await sleep(1000) }
  }
  if (!at) throw new Error('admin 登录失败（server 未就绪？）')

  // ---------- 数据准备 ----------
  const mkQ = (title, mode, extra = {}) => j('POST', '/api/admin/quiz', { title, mode, per_question_time: 60, rush_time: 15, rush_answer_time: 20, rush_bonus_score: 5, ...extra }, at)
  const mkQs = (id, q) => j('POST', `/api/admin/quiz/${id}/questions`, q, at)
  const opts = labels => labels.map((l, i) => ({ label: l, content: `选项${l}` }))

  const quizR = (await mkQ('sec-rush', 'rush', { show_answer: true, show_analysis: true, show_ranking: true })).data
  const qR = (await mkQs(quizR.id, { type: 'single', content: '抢答题?', answer: 'B', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const quizN = (await mkQ('sec-normal-hidden', 'normal', { show_answer: false, show_analysis: false, show_ranking: true })).data
  const qN = (await mkQs(quizN.id, { type: 'single', content: '隐藏题?', answer: 'A', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const quizF = (await mkQ('sec-normal-show', 'normal', { show_answer: true, show_analysis: true, show_ranking: true })).data
  const qF = (await mkQs(quizF.id, { type: 'single', content: '展示题?', answer: 'B', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const qM = (await mkQs(quizF.id, { type: 'multiple', content: '多选?', answer: 'AC', score: 10, required: true, time_limit: 20, options: opts(['A', 'B', 'C']) })).data

  const reg = (u, n) => j('POST', '/api/auth/register', { username: u, password: 'test-pass-1234', nickname: n })
  const alice = (await reg('sec_alice', 'Alice')).data
  const bob = (await reg('sec_bob', 'Bob')).data
  const eve = (await reg('sec_eve', 'Eve')).data
  const jR_a = (await j('POST', '/api/join', { quiz_id: quizR.id }, alice.token)).data
  const jR_b = (await j('POST', '/api/join', { quiz_id: quizR.id }, bob.token)).data
  const jN_a = (await j('POST', '/api/join', { quiz_id: quizN.id }, alice.token)).data
  const jF_a = (await j('POST', '/api/join', { quiz_id: quizF.id }, alice.token)).data
  const jF_b = (await j('POST', '/api/join', { quiz_id: quizF.id }, bob.token)).data

  // ---------- 1. 状态机 / 越权 / 答案不下发（普通题未开始） ----------
  const wait1 = await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, jN_a.token)
  check('C2 未开始(WAITING)：提交被拒', wait1.code !== 0, `code=${wait1.code}`)
  const adminAsUser = await j('GET', '/api/admin/quizzes', null, jN_a.token)
  check('C1 用户 token 调 admin API 被拒', adminAsUser.code !== 0, `code=${adminAsUser.code}`)
  const noJoin = await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, eve.token)
  check('C3 未参加者提交被拒', noJoin.code !== 0, `code=${noJoin.code}`)

  // ---------- 2. 抢答权限（问题1） ----------
  await j('POST', `/api/admin/quiz/${quizR.id}/start`, {}, at)
  const rushPhaseSubmit = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 100 }, jR_a.token)
  check('A1 抢答阶段(RUSHING)普通提交被拒', rushPhaseSubmit.code !== 0, `code=${rushPhaseSubmit.code}`)

  await j('POST', `/api/admin/quiz/${quizR.id}/rush/start`, {}, at)
  await sleep(300)
  const r1 = await j('POST', `/api/question/${qR.id}/rush`, {}, jR_a.token) // 仅 Alice 抢
  check('A2 Alice 抢答成功 rank=1', r1.code === 0 && r1.data?.rank === 1, `code=${r1.code} rank=${r1.data?.rank}`)
  await j('POST', `/api/admin/quiz/${quizR.id}/rush/end`, {}, at)
  await sleep(300)
  const bobTry = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 100 }, jR_b.token)
  check('A3 未抢到的 Bob 提交被拒(核心)', bobTry.code !== 0 && /资格/.test(bobTry.msg || ''), `code=${bobTry.code} msg=${bobTry.msg}`)
  const bobRushAgain = await j('POST', `/api/question/${qR.id}/rush`, {}, jR_b.token)
  check('A4 抢答窗口关闭后 Bob 再抢被拒', bobRushAgain.code !== 0, `code=${bobRushAgain.code}`)
  const aliceAns = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 300 }, jR_a.token)
  check('A5 抢到的 Alice 可提交', aliceAns.code === 0, `code=${aliceAns.code}`)
  const curR = JSON.stringify(await j('GET', `/api/quiz/${quizR.id}/current-question`, null, jR_a.token))
  check('B1 抢答题 current-question 无答案', !leak(curR), '')

  // ---------- 3. reveal 门控（问题2） ----------
  // 3a. show_answer=false：reveal 不含 correct_answer/analysis（reveal 前先挂 WS 收广播）
  const revealsN = []
  const wsN = await connect(jN_a.token, m => { if (m.event === 'answer:reveal') revealsN.push(m.data) })
  await j('POST', `/api/admin/quiz/${quizN.id}/start`, {}, at)
  await sleep(200)
  await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, jN_a.token)
  await j('POST', `/api/admin/quiz/${quizN.id}/reveal`, {}, at)
  await sleep(600)
  check('B4 show_answer=false：reveal 无 correct_answer/analysis', revealsN.length > 0 && revealsN.every(r => !r.correct_answer && !r.analysis), `events=${revealsN.length} first=${JSON.stringify(revealsN[0] || {}).slice(0, 90)}`)
  wsN.close()

  // 3b. show_answer=true：reveal 含正确答案 + 个人单播各拿各的；即时结果不含正确答案
  let resultA = null, resultB = null
  const revealsA = [], revealsB = []
  const wfA = await connect(jF_a.token, m => { if (m.event === 'answer:result') resultA = m.data; if (m.event === 'answer:reveal') revealsA.push(m.data) })
  const wfB = await connect(jF_b.token, m => { if (m.event === 'answer:result') resultB = m.data; if (m.event === 'answer:reveal') revealsB.push(m.data) })
  await j('POST', `/api/admin/quiz/${quizF.id}/start`, {}, at)
  await sleep(200)
  await j('POST', `/api/question/${qF.id}/answer`, { answer: 'B', duration: 100 }, jF_a.token) // Alice 对
  await j('POST', `/api/question/${qF.id}/answer`, { answer: 'A', duration: 100 }, jF_b.token) // Bob 错
  await sleep(400)
  check('B2 即时 result 不含正确答案(只回显本人答案)', !!resultA && !!resultB && !leakSecret(JSON.stringify({ resultA, resultB })), `A=${resultA?.answer} B=${resultB?.answer}`)
  await j('POST', `/api/admin/quiz/${quizF.id}/reveal`, {}, at)
  await sleep(600)
  const mineA = revealsA.find(r => r.my_answer !== undefined), mineB = revealsB.find(r => r.my_answer !== undefined)
  check('B5 show_answer=true：reveal 单播含 correct_answer', mineA?.correct_answer === 'B' && mineB?.correct_answer === 'B', `A=${mineA?.correct_answer} B=${mineB?.correct_answer}`)
  check('B6 reveal 单播个人答案各拿各的', mineA?.my_answer === 'B' && mineB?.my_answer === 'A', `A=${mineA?.my_answer} B=${mineB?.my_answer}`)
  check('B7 reveal 所有事件不含他人答案(公共广播无个人字段外泄)', revealsA.every(r => r.my_answer === undefined || r.my_answer === 'B') && revealsB.every(r => r.my_answer === undefined || r.my_answer === 'A'), '')
  wfA.close(); wfB.close()

  // ---------- 4. 多选乱序 / 非法选项 / 结束后 / 倒计时超时 ----------
  const quizS = (await mkQ('sec-misc', 'normal', { show_answer: true })).data
  const qS1 = (await mkQs(quizS.id, { type: 'multiple', content: '多选?', answer: 'AC', score: 10, required: true, time_limit: 20, options: opts(['A', 'B', 'C']) })).data
  const qS2 = (await mkQs(quizS.id, { type: 'single', content: '单选?', answer: 'A', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const jS_a = (await j('POST', '/api/join', { quiz_id: quizS.id }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizS.id}/start`, {}, at)
  await sleep(300)
  const multiCA = await j('POST', `/api/question/${qS1.id}/answer`, { answer: 'CA', duration: 100 }, jS_a.token) // 乱序提交 AC
  check('C5 多选乱序(CA==AC)判对', multiCA.code === 0 && multiCA.data?.is_correct === true, `code=${multiCA.code} correct=${multiCA.data?.is_correct}`)
  await j('POST', `/api/admin/quiz/${quizS.id}/next`, {}, at); await sleep(300) // 切到第二题
  const badOpt = await j('POST', `/api/question/${qS2.id}/answer`, { answer: 'Z', duration: 100 }, jS_a.token)
  check('C6 非法选项被拒', badOpt.code !== 0 && /不合法/.test(badOpt.msg || ''), `code=${badOpt.code} msg=${badOpt.msg}`)
  await j('POST', `/api/admin/quiz/${quizS.id}/end`, {}, at)
  const afterEnd = await j('POST', `/api/question/${qS2.id}/answer`, { answer: 'A', duration: 100 }, jS_a.token)
  check('C7 结束后(FINISHED)提交被拒', afterEnd.code !== 0, `code=${afterEnd.code}`)

  const quizT = (await mkQ('sec-timeout', 'normal', { show_answer: true })).data
  const qT = (await mkQs(quizT.id, { type: 'single', content: '超时?', answer: 'A', score: 10, required: true, time_limit: 1, options: opts(['A', 'B']) })).data
  const jT = (await j('POST', '/api/join', { quiz_id: quizT.id }, bob.token)).data
  await j('POST', `/api/admin/quiz/${quizT.id}/start`, {}, at)
  await sleep(4000) // 1s 题 + 1.5s 宽限
  const late = await j('POST', `/api/question/${qT.id}/answer`, { answer: 'A', duration: 100 }, jT.token)
  check('C8 倒计时超时(含宽限)后提交被拒', late.code !== 0, `code=${late.code} msg=${late.msg}`)
  await j('POST', `/api/admin/quiz/${quizT.id}/end`, {}, at)

  for (const id of [quizR.id, quizN.id, quizF.id, quizS.id, quizT.id]) await j('DELETE', `/api/admin/quiz/${id}`, null, at)
  console.log(`\n${fail === 0 ? 'ALL PASS' : 'HAS FAILURES'}: ${pass} passed, ${fail} failed`)
  process.exit(fail === 0 ? 0 : 1)
})().catch(e => { console.error('FAIL:', e.message); process.exit(1) })
