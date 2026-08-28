// 理论答题安全 E2E：抢答权限 / 答案回显 / reveal 门控 / 状态机 / 越权 / 倒计时
// 用法：node scripts/security_e2e.mjs   （默认不清库；CLEAN=1 时先清库）
// 开头清空 MySQL 各表 + Redis 抢答状态，再建测试用户与题目；任一断言失败退出码 1
import { execSync as sh } from 'node:child_process'
const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
import { readFileSync } from 'node:fs'
const env = k => process.env[k] || (readFileSync('.env', 'utf8').match(new RegExp('^' + k + '=(.*)$', 'm')) || [])[1] || ''
const MYSQL_PASS = env('MYSQL_ROOT_PASSWORD')
const REDIS_PASS = env('REDIS_PASSWORD')
const ADMIN_PASS = env('ADMIN_PASS')

const j = async (m, u, b, tok) => {
  const r = await fetch(B + u, { method: m, headers: { 'Content-Type': 'application/json', ...(tok ? { Authorization: 'Bearer ' + tok } : {}) }, body: b ? JSON.stringify(b) : undefined })
  return r.json()
}
const sleep = ms => new Promise(r => setTimeout(r, ms))
let pass = 0, fail = 0
const check = (name, ok, extra = '') => { ok ? pass++ : fail++; console.log(`${ok ? '✅' : '❌'} ${name}${extra ? '  ' + extra : ''}`) }
const leakSecret = s => /"correct_answer"\s*:\s*"[^"]+"/.test(s) || /"analysis"\s*:\s*"[^"]+"/.test(s)
const leak = s => leakSecret(s) || /"answer"\s*:\s*"[A-D]"/.test(s)
const createdQuizzes = [], createdUsers = [] // 本场创建的资源，结尾统一清理

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
  let at = null
  try {
  // ---------- 0. 清理数据库 + Redis（仅 CLEAN=1 时执行，保护测试/模拟数据） ----------
  if (process.env.CLEAN === '1') {
    try {
      sh(`docker exec kaoshi-mysql mysql -uroot -p${MYSQL_PASS} kaoshi -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE quiz_invitees; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"`, { stdio: ['ignore', 'pipe', 'pipe'] })
      sh(`docker exec kaoshi-redis redis-cli -a ${REDIS_PASS} FLUSHDB`, { stdio: ['ignore', 'pipe', 'pipe'] })
      // 自增 ID 复用会撞上 server 内存里的旧 Runtime，重启清空（本机容器场景）
      try { sh('docker restart kaoshi-server', { stdio: ['ignore', 'pipe', 'pipe'] }) } catch {}
      console.log('🗑  已清库（MySQL 各表 + Redis 抢答状态 + server Runtime）')
    } catch (e) {
      console.log('⚠ 清理失败（本机无容器？）', String(e).split('\n')[0])
    }
  }

  for (let i = 0; i < 30 && !at; i++) {
    try { at = (await j('POST', '/api/admin/login', { username: 'admin', password: ADMIN_PASS })).data.token } catch { await sleep(1000) }
  }
  if (!at) throw new Error('admin 登录失败（server 未就绪？）')
  // ---------- 数据准备 ----------
  const mkQ = async (title, mode, extra = {}) => {
    const r = await j('POST', '/api/admin/quiz', { title, mode, per_question_time: 60, rush_time: 15, rush_answer_time: 20, rush_bonus_score: 5, ...extra }, at)
    if (r.code === 0) createdQuizzes.push(r.data.code)
    return r
  }
  const mkQs = (id, q) => j('POST', `/api/admin/quiz/${id}/questions`, q, at)
  const opts = labels => labels.map((l, i) => ({ label: l, content: `选项${l}` }))

  const quizR = (await mkQ('sec-rush', 'rush', { show_answer: true, show_analysis: true, show_ranking: true, rush_countdown: 0 })).data
  const qR = (await mkQs(quizR.code, { type: 'single', content: '抢答题?', answer: 'B', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const quizN = (await mkQ('sec-normal-hidden', 'normal', { show_answer: false, show_analysis: false, show_ranking: true })).data
  const qN = (await mkQs(quizN.code, { type: 'single', content: '隐藏题?', answer: 'A', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const quizF = (await mkQ('sec-normal-show', 'normal', { show_answer: true, show_analysis: true, show_ranking: true })).data
  const qF = (await mkQs(quizF.code, { type: 'single', content: '展示题?', answer: 'B', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const qM = (await mkQs(quizF.code, { type: 'multiple', content: '多选?', answer: 'AC', score: 10, required: true, time_limit: 20, options: opts(['A', 'B', 'C']) })).data

  const reg = async (u, n) => { // 注册已下线：admin 建号 + 登录
    await j('POST', '/api/admin/users', { username: u, password: 'test-pass-1234', nickname: n }, at)
    createdUsers.push(u)
    return (await j('POST', '/api/auth/login', { username: u, password: 'test-pass-1234' })).data
  }
  const sfx = Date.now() % 100000
  const alice = await reg(`zhangwei${sfx}`, '张伟')
  const bob = await reg(`lina${sfx}`, '李娜')
  const eve = await reg(`wangfang${sfx}`, '王芳')
  const jR_a = (await j('POST', '/api/join', { quiz_id: quizR.code }, alice.token)).data
  const jR_b = (await j('POST', '/api/join', { quiz_id: quizR.code }, bob.token)).data
  const jN_a = (await j('POST', '/api/join', { quiz_id: quizN.code }, alice.token)).data
  const jF_a = (await j('POST', '/api/join', { quiz_id: quizF.code }, alice.token)).data
  const jF_b = (await j('POST', '/api/join', { quiz_id: quizF.code }, bob.token)).data

  // ---------- 1. 状态机 / 越权 / 答案不下发（普通题未开始） ----------
  const wait1 = await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, jN_a.token)
  check('C2 未开始(WAITING)：提交被拒', wait1.code !== 0, `code=${wait1.code}`)
  const adminAsUser = await j('GET', '/api/admin/quizzes', null, jN_a.token)
  check('C1 用户 token 调 admin API 被拒', adminAsUser.code !== 0, `code=${adminAsUser.code}`)
  const noJoin = await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, eve.token)
  check('C3 未参加者提交被拒', noJoin.code !== 0, `code=${noJoin.code}`)

  // ---------- 1a. 失效身份：用户被删后旧 token 提交必须被拒（防孤儿答案/总分 0）
  const ghost = await reg(`ghost${sfx}`, '幽灵')
  const jGrace = (await j('POST', '/api/join', { quiz_id: quizN.code }, ghost.token)).data
  await j('POST', `/api/admin/quiz/${quizN.code}/start`, {}, at); await sleep(300)
  const uId = JSON.parse(Buffer.from(ghost.token.split('.')[1], 'base64').toString()).uid
  await j('DELETE', `/api/admin/users/${uId}`, null, at) // 删用户（级联 participants/answers）
  const ghostAns = await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, jGrace.token)
  check('C9 失效 token（用户已删）提交被拒', ghostAns.code !== 0, `code=${ghostAns.code} msg=${ghostAns.msg}`)
  const ghostRush = await j('POST', `/api/question/${qN.id}/rush`, {}, jGrace.token)
  check('C10 失效 token 抢答被拒', ghostRush.code !== 0, `code=${ghostRush.code} msg=${ghostRush.msg}`)
  await j('POST', `/api/admin/quiz/${quizN.code}/reset`, {}, at) // 恢复 WAITING 供后续用例

  // ---------- 1b. 普通模式混合题：抢答题在抢答前不可直接作答（防绕过抢答） ----------
  const quizX = (await mkQ('sec-mixed', 'normal', { rush_enabled: true, rush_time: 15, rush_answer_time: 20, rush_deduct_single: 4, rush_countdown: 0 })).data
  const qX = (await mkQs(quizX.code, { type: 'single', content: '混合抢答题?', answer: 'B', score: 10, required: false, time_limit: 20, options: opts(['A', 'B']) })).data
  const qX2 = (await mkQs(quizX.code, { type: 'single', content: '混合抢答题2?', answer: 'A', score: 10, required: false, time_limit: 20, options: opts(['A', 'B']) })).data
  const jX_a = (await j('POST', '/api/join', { quiz_id: quizX.code }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizX.code}/start`, {}, at)
  await sleep(300)
  const xPreRush = await j('POST', `/api/question/${qX.id}/answer`, { answer: 'B', duration: 100 }, jX_a.token)
  check('X1 开窗前直接提交被拒(核心)', xPreRush.code !== 0 && /资格|不可作答/.test(xPreRush.msg || ''), `code=${xPreRush.code} msg=${xPreRush.msg}`)
  await j('POST', `/api/admin/quiz/${quizX.code}/rush/start`, {}, at)
  await sleep(300)
  const xDuringRush = await j('POST', `/api/question/${qX.id}/answer`, { answer: 'B', duration: 100 }, jX_a.token)
  check('X2 窗口开启中未抢先答被拒', xDuringRush.code !== 0, `code=${xDuringRush.code} msg=${xDuringRush.msg}`)
  const xRush = await j('POST', `/api/question/${qX.id}/rush`, {}, jX_a.token)
  check('X3 Alice 抢答成功', xRush.code === 0 && xRush.data?.rank === 1, `code=${xRush.code}`)
  await j('POST', `/api/admin/quiz/${quizX.code}/rush/end`, {}, at)
  await sleep(300)
  const xAfterRush = await j('POST', `/api/question/${qX.id}/answer`, { answer: 'B', duration: 100 }, jX_a.token)
  check('X4 抢到后可提交', xAfterRush.code === 0, `code=${xAfterRush.code} msg=${xAfterRush.msg}`)
  check('S1 抢答答对=题目分值无奖励', xAfterRush.data?.score === 10 && xAfterRush.data?.total_score === 10, `score=${xAfterRush.data?.score} total=${xAfterRush.data?.total_score}`)
  // S2：第二道抢答题，抢到后答错 → 0 分不倒扣
  await j('POST', `/api/admin/quiz/${quizX.code}/next`, {}, at); await sleep(300)
  await j('POST', `/api/admin/quiz/${quizX.code}/rush/start`, {}, at); await sleep(300)
  await j('POST', `/api/question/${qX2.id}/rush`, {}, jX_a.token)
  await j('POST', `/api/admin/quiz/${quizX.code}/rush/end`, {}, at); await sleep(300)
  const xWrong = await j('POST', `/api/question/${qX2.id}/answer`, { answer: 'B', duration: 100 }, jX_a.token)
  check('S2 抢答答错按题型扣分(-4)', xWrong.code === 0 && xWrong.data?.score === -4 && xWrong.data?.total_score === 6, `score=${xWrong.data?.score} total=${xWrong.data?.total_score}`)

  // ---------- 1d. 抢答开抢倒计时（服务端 open_at；防 API 直连绕过前端 3s 动画） ----------
  const quizC = (await mkQ('sec-countdown', 'normal', { rush_enabled: true, rush_time: 15, rush_answer_time: 20 })).data // 默认 rush_countdown=3
  const qC = (await mkQs(quizC.code, { type: 'single', content: '倒计时抢答题?', answer: 'A', score: 10, required: false, time_limit: 20, options: opts(['A', 'B']) })).data
  const jC_a = (await j('POST', '/api/join', { quiz_id: quizC.code }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizC.code}/start`, {}, at); await sleep(300)
  await j('POST', `/api/admin/quiz/${quizC.code}/rush/start`, {}, at)
  const earlyRush = await j('POST', `/api/question/${qC.id}/rush`, {}, jC_a.token)
  check('X18 开抢倒计时内 API 抢答被拒', earlyRush.code !== 0 && /尚未开始/.test(earlyRush.msg || ''), `code=${earlyRush.code} msg=${earlyRush.msg}`)
  await sleep(3200)
  const onTimeRush = await j('POST', `/api/question/${qC.id}/rush`, {}, jC_a.token)
  check('X19 倒计时结束后抢答成功', onTimeRush.code === 0 && onTimeRush.data?.rank === 1, `code=${onTimeRush.code} msg=${onTimeRush.msg}`)

  // ---------- 1c. 邀请制 + 我的比赛（quiz_invitees，X 系列） ----------
  const quizV = (await mkQ('sec-invite', 'normal', { show_ranking: true })).data // 受限赛
  const quizO = (await mkQ('sec-open', 'normal', {})).data // 开放赛（名单为空）
  const qV = (await mkQs(quizV.code, { type: 'single', content: '邀请制题?', answer: 'A', score: 10, required: true, time_limit: 30, options: opts(['A', 'B']) })).data
  // X5 名单设置与读取（重复 id 幂等去重）
  const putInv = await j('PUT', `/api/admin/quiz/${quizV.code}/invitees`, { user_ids: [alice.user.id, alice.user.id, bob.user.id] }, at)
  const getInv = await j('GET', `/api/admin/quiz/${quizV.code}/invitees`, null, at)
  const invIds = (getInv.data?.items || []).map(i => i.user_id).sort().join(',')
  check('X5 名单设置与读取(含去重)', putInv.code === 0 && getInv.code === 0 && invIds === [alice.user.id, bob.user.id].sort().join(',') && getInv.data.items.every(i => i.username && i.nickname), `put=${putInv.code} ids=${invIds}`)
  // X12 越权：用户 token 调 admin 接口
  const invByUser = await j('GET', `/api/admin/quiz/${quizV.code}/invitees`, null, alice.token)
  check('X12 用户调名单接口被拒', invByUser.code === 401, `code=${invByUser.code}`)
  // X6 受邀者可加入
  const jV_b = (await j('POST', '/api/join', { quiz_id: quizV.code }, bob.token)).data
  check('X6 受邀者可加入', jV_b.token && jV_b.quiz?.code === quizV.code, `code=${jV_b.code}`)
  // X7 未受邀者被拒；直链 brief 仍公开
  const jV_e = await j('POST', '/api/join', { quiz_id: quizV.code }, eve.token)
  const briefV = await j('GET', `/api/quiz/${quizV.code}/brief`)
  check('X7 未受邀者被拒(403)/brief公开', jV_e.code === 403 && briefV.code === 0, `join=${jV_e.code} brief=${briefV.code}`)
  // X8 列表可见性：eve 看不到受限赛，bob 能看到；X13 joined 标记
  const listE = await j('GET', '/api/quizzes', null, eve.token)
  const listB = await j('GET', '/api/quizzes', null, bob.token)
  const idsE = (listE.data?.items || []).map(i => i.id)
  const rowB = (listB.data?.items || []).find(i => i.code === quizV.code)
  check('X8 受限赛对未受邀者不可见', !idsE.includes(quizV.code) && !!rowB, `eve sees=${idsE.includes(quizV.code)}`)
  check('X13 已加入标记 joined=true', rowB?.joined === true, `joined=${rowB?.joined}`)
  // X9 开放回归：名单为空任何人可加入
  const jO_e = (await j('POST', '/api/join', { quiz_id: quizO.code }, eve.token)).data
  check('X9 空名单开放可加入', jO_e.token && jO_e.quiz?.code === quizO.code, `code=${jO_e.code}`)
  // X14 我的比赛-进行中：alice 打一场（受限赛，受邀）
  const jV_a = (await j('POST', '/api/join', { quiz_id: quizV.code }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizV.code}/start`, {}, at); await sleep(300)
  const myMid = await j('GET', '/api/my/quizzes', null, alice.token)
  const rowV = (myMid.data?.items || []).find(i => i.code === quizV.code)
  check('X14 我的比赛含进行中', rowV && ['RUNNING', 'ANSWERING', 'RUSHING', 'PAUSED', 'REVEALING'].includes(rowV.status), `status=${rowV?.status}`)
  await j('POST', `/api/question/${qV.id}/answer`, { answer: 'A', duration: 100 }, jV_a.token)
  const myMid2 = await j('GET', '/api/my/quizzes', null, alice.token)
  const rowV2 = (myMid2.data?.items || []).find(i => i.code === quizV.code)
  check('X14b 我的比赛分数实时', rowV2?.score === 10, `score=${rowV2?.score}`)
  // X10 状态限制：RUNNING 不能改名单
  const putRun = await j('PUT', `/api/admin/quiz/${quizV.code}/invitees`, { user_ids: [alice.user.id] }, at)
  check('X10 RUNNING 改名单被拒', putRun.code !== 0, `code=${putRun.code}`)
  // X15/X16 已结束：结束赛在我的列表且分数正确；未参加者无；重入拿 token
  await j('POST', `/api/admin/quiz/${quizV.code}/end`, {}, at)
  const myEnd = await j('GET', '/api/my/quizzes', null, alice.token)
  const rowVE = (myEnd.data?.items || []).find(i => i.code === quizV.code)
  const myEndE = await j('GET', '/api/my/quizzes', null, eve.token)
  const eveHas = (myEndE.data?.items || []).some(i => i.code === quizV.code)
  check('X15 已结束在我的列表+分数', rowVE?.status === 'FINISHED' && rowVE?.score === 10 && !eveHas, `status=${rowVE?.status} score=${rowVE?.score} eveHas=${eveHas}`)
  const reA = await j('POST', '/api/join', { quiz_id: quizV.code }, alice.token)
  const reE = await j('POST', '/api/join', { quiz_id: quizV.code }, eve.token)
  check('X16 参与者可重入/未参与者不可', reA.code === 0 && reE.code !== 0, `reA=${reA.code} reE=${reE.code}`)
  // X11 不存在用户：整单拒绝，原名单不变
  const putBad = await j('PUT', `/api/admin/quiz/${quizO.code}/invitees`, { user_ids: [999999] }, at)
  const getO = await j('GET', `/api/admin/quiz/${quizO.code}/invitees`, null, at)
  check('X11 不存在用户整单拒绝', putBad.code !== 0 && (getO.data?.items || []).length === 0, `put=${putBad.code} n=${(getO.data?.items || []).length}`)
  // X17 无 token 访问 my 接口
  const myNoAuth = await j('GET', '/api/my/quizzes')
  check('X17 无 token 访问我的比赛被拒', myNoAuth.code === 401, `code=${myNoAuth.code}`)

  // ---------- 2. 抢答权限（问题1） ----------
  await j('POST', `/api/admin/quiz/${quizR.code}/start`, {}, at)
  const rushPhaseSubmit = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 100 }, jR_a.token)
  check('A1 抢答阶段(RUSHING)普通提交被拒', rushPhaseSubmit.code !== 0, `code=${rushPhaseSubmit.code}`)

  await j('POST', `/api/admin/quiz/${quizR.code}/rush/start`, {}, at)
  await sleep(300)
  const r1 = await j('POST', `/api/question/${qR.id}/rush`, {}, jR_a.token) // 仅 Alice 抢
  check('A2 Alice 抢答成功 rank=1 且无奖励分', r1.code === 0 && r1.data?.rank === 1 && (r1.data?.bonus ?? 0) === 0 && (r1.data?.score ?? 0) === 0, `code=${r1.code} rank=${r1.data?.rank} bonus=${r1.data?.bonus} score=${r1.data?.score}`)
  await j('POST', `/api/admin/quiz/${quizR.code}/rush/end`, {}, at)
  await sleep(300)
  const bobTry = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 100 }, jR_b.token)
  check('A3 未抢到的 Bob 提交被拒(核心)', bobTry.code !== 0 && /资格/.test(bobTry.msg || ''), `code=${bobTry.code} msg=${bobTry.msg}`)
  const bobRushAgain = await j('POST', `/api/question/${qR.id}/rush`, {}, jR_b.token)
  check('A4 抢答窗口关闭后 Bob 再抢被拒', bobRushAgain.code !== 0, `code=${bobRushAgain.code}`)
  const aliceAns = await j('POST', `/api/question/${qR.id}/answer`, { answer: 'B', duration: 300 }, jR_a.token)
  check('A5 抢到的 Alice 可提交', aliceAns.code === 0, `code=${aliceAns.code}`)
  check('S4 抢到的答对=题目分值无奖励(quizR)', aliceAns.data?.score === 10 && aliceAns.data?.total_score === 10, `score=${aliceAns.data?.score} total=${aliceAns.data?.total_score}`)
  const curR = JSON.stringify(await j('GET', `/api/quiz/${quizR.code}/current-question`, null, jR_a.token))
  check('B1 抢答题 current-question 无答案', !leak(curR), '')

  // ---------- 3. reveal 门控（问题2） ----------
  // 3a. show_answer=false：reveal 不含 correct_answer/analysis（reveal 前先挂 WS 收广播）
  const revealsN = []
  const wsN = await connect(jN_a.token, m => { if (m.event === 'answer:reveal') revealsN.push(m.data) })
  await j('POST', `/api/admin/quiz/${quizN.code}/start`, {}, at)
  await sleep(200)
  await j('POST', `/api/question/${qN.id}/answer`, { answer: 'A', duration: 100 }, jN_a.token)
  await j('POST', `/api/admin/quiz/${quizN.code}/reveal`, {}, at)
  await sleep(600)
  check('B4 show_answer=false：reveal 无 correct_answer/analysis', revealsN.length > 0 && revealsN.every(r => !r.correct_answer && !r.analysis), `events=${revealsN.length} first=${JSON.stringify(revealsN[0] || {}).slice(0, 90)}`)
  wsN.close()

  // 3b. show_answer=true：reveal 含正确答案 + 个人单播各拿各的；即时结果不含正确答案
  let resultA = null, resultB = null
  const revealsA = [], revealsB = []
  const wfA = await connect(jF_a.token, m => { if (m.event === 'answer:result') resultA = m.data; if (m.event === 'answer:reveal') revealsA.push(m.data) })
  const wfB = await connect(jF_b.token, m => { if (m.event === 'answer:result') resultB = m.data; if (m.event === 'answer:reveal') revealsB.push(m.data) })
  await j('POST', `/api/admin/quiz/${quizF.code}/start`, {}, at)
  await sleep(200)
  await j('POST', `/api/question/${qF.id}/answer`, { answer: 'B', duration: 100 }, jF_a.token) // Alice 对
  await j('POST', `/api/question/${qF.id}/answer`, { answer: 'A', duration: 100 }, jF_b.token) // Bob 错
  await sleep(400)
  check('B2 即时 result 不含正确答案(只回显本人答案)', !!resultA && !!resultB && !leakSecret(JSON.stringify({ resultA, resultB })), `A=${resultA?.answer} B=${resultB?.answer}`)
  await j('POST', `/api/admin/quiz/${quizF.code}/reveal`, {}, at)
  await sleep(600)
  const mineA = revealsA.find(r => r.my_answer !== undefined), mineB = revealsB.find(r => r.my_answer !== undefined)
  check('B5 show_answer=true：reveal 单播含 correct_answer', mineA?.correct_answer === 'B' && mineB?.correct_answer === 'B', `A=${mineA?.correct_answer} B=${mineB?.correct_answer}`)
  check('B6 reveal 单播个人答案各拿各的', mineA?.my_answer === 'B' && mineB?.my_answer === 'A', `A=${mineA?.my_answer} B=${mineB?.my_answer}`)
  check('B7 reveal 所有事件不含他人答案(公共广播无个人字段外泄)', revealsA.every(r => r.my_answer === undefined || r.my_answer === 'B') && revealsB.every(r => r.my_answer === undefined || r.my_answer === 'A'), '')
  wfA.close(); wfB.close()

  // ---------- 4. 多选乱序 / 非法选项 / 结束后 / 倒计时超时 ----------
  const quizS = (await mkQ('sec-misc', 'normal', { show_answer: true })).data
  const qS1 = (await mkQs(quizS.code, { type: 'multiple', content: '多选?', answer: 'AC', score: 10, required: true, time_limit: 20, options: opts(['A', 'B', 'C']) })).data
  const qS2 = (await mkQs(quizS.code, { type: 'single', content: '单选?', answer: 'A', score: 10, required: true, time_limit: 20, options: opts(['A', 'B']) })).data
  const jS_a = (await j('POST', '/api/join', { quiz_id: quizS.code }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizS.code}/start`, {}, at)
  await sleep(300)
  const multiCA = await j('POST', `/api/question/${qS1.id}/answer`, { answer: 'CA', duration: 100 }, jS_a.token) // 乱序提交 AC
  check('C5 多选乱序(CA==AC)判对', multiCA.code === 0 && multiCA.data?.is_correct === true, `code=${multiCA.code} correct=${multiCA.data?.is_correct}`)
  await j('POST', `/api/admin/quiz/${quizS.code}/next`, {}, at); await sleep(300) // 切到第二题
  const badOpt = await j('POST', `/api/question/${qS2.id}/answer`, { answer: 'Z', duration: 100 }, jS_a.token)
  check('C6 非法选项被拒', badOpt.code !== 0 && /不合法/.test(badOpt.msg || ''), `code=${badOpt.code} msg=${badOpt.msg}`)
  await j('POST', `/api/admin/quiz/${quizS.code}/end`, {}, at)
  const afterEnd = await j('POST', `/api/question/${qS2.id}/answer`, { answer: 'A', duration: 100 }, jS_a.token)
  check('C7 结束后(FINISHED)提交被拒', afterEnd.code !== 0, `code=${afterEnd.code}`)

  const quizT = (await mkQ('sec-timeout', 'normal', { show_answer: true })).data
  const qT = (await mkQs(quizT.code, { type: 'single', content: '超时?', answer: 'A', score: 10, required: true, time_limit: 1, options: opts(['A', 'B']) })).data
  const jT = (await j('POST', '/api/join', { quiz_id: quizT.code }, bob.token)).data
  await j('POST', `/api/admin/quiz/${quizT.code}/start`, {}, at)
  await sleep(4000) // 1s 题 + 1.5s 宽限
  const late = await j('POST', `/api/question/${qT.id}/answer`, { answer: 'A', duration: 100 }, jT.token)
  check('C8 倒计时超时(含宽限)后提交被拒', late.code !== 0, `code=${late.code} msg=${late.msg}`)
  await j('POST', `/api/admin/quiz/${quizT.code}/end`, {}, at)

  // C11 到点宽限内补交被接受：前端「时间到自动补交已选答案」的服务端契约。
  // 收卷定时器同样延后 1.5s：deadline+~0.3s 的在途提交必须成功，且后续收卷不得把它覆盖为“未答”。
  const quizC11 = (await mkQ('sec-grace', 'normal', { show_answer: true })).data
  const qC11 = (await mkQs(quizC11.code, { type: 'single', content: '宽限?', answer: 'A', score: 10, required: true, time_limit: 2, options: opts(['A', 'B']) })).data
  const jC11 = (await j('POST', '/api/join', { quiz_id: quizC11.code }, bob.token)).data
  await j('POST', `/api/admin/quiz/${quizC11.code}/start`, {}, at)
  await sleep(2300) // 2s 题 + ~0.3s：模拟客户端到点自动补交
  const ingrace = await j('POST', `/api/question/${qC11.id}/answer`, { answer: 'A', duration: 100 }, jC11.token)
  check('C11 到点宽限(1.5s)内补交被接受', ingrace.code === 0 && ingrace.data?.is_correct === true, `code=${ingrace.code} msg=${ingrace.msg}`)
  await sleep(1500) // 越过宽限，等服务端收卷完成
  await j('POST', `/api/admin/quiz/${quizC11.code}/end`, {}, at)
  const resC11 = (await j('GET', `/api/quiz/${quizC11.code}/result`, null, jC11.token)).data
  check('C11b 收卷不覆盖已补交答案', resC11.correct === 1 && resC11.wrong === 0, JSON.stringify(resC11))

  // ---------- 5. 排行榜权限（考试模式排行榜仅管理员可见） ----------
  const quizE = (await mkQ('sec-exam-rank', 'exam', { total_time: 300, show_ranking: true })).data
  const qE = (await mkQs(quizE.code, { type: 'single', content: '考试榜?', answer: 'A', score: 10, required: true, time_limit: 30, options: opts(['A', 'B']) })).data
  const jE = (await j('POST', '/api/join', { quiz_id: quizE.code }, alice.token)).data
  await j('POST', `/api/admin/quiz/${quizE.code}/start`, {}, at)
  await j('POST', `/api/quiz/${quizE.code}/paper/answer`, { question_id: qE.id, answer: 'A', duration: 100 }, jE.token)
  await sleep(100)
  const uRankE = await j('GET', `/api/quiz/${quizE.code}/ranking`, null, jE.token)
  check('D1 考试模式选手拉排行榜被拒(仅管理员)', uRankE.code !== 0, `code=${uRankE.code} msg=${uRankE.msg}`)
  const uStatsE = await j('GET', `/api/admin/quiz/${quizE.code}/statistics`, null, jE.token)
  check('D2 用户 token 访问管理统计接口被拒', uStatsE.code !== 0, `code=${uStatsE.code}`)
  const noTokE = await fetch(`${B}/api/admin/quiz/${quizE.code}/statistics`).then(r => r.status)
  check('D3 无 token 访问管理统计接口被拒(401)', noTokE === 401, `http=${noTokE}`)
  // 实时排行榜本体（管理员视角）：未交卷也有分
  const liveStats = (await j('GET', `/api/admin/quiz/${quizE.code}/statistics`, null, at)).data
  const mineE = (liveStats.ranking || []).find(r => r.user_id === alice.user.id)
  check('D4 管理端实时排行榜：未交卷也计分', mineE?.score === 10 && mineE?.rank === 1, JSON.stringify(mineE))
  // 试卷接口不泄露排行榜/他人数据：paper 只含本人答案与题面
  const paperStr = JSON.stringify(await j('GET', `/api/quiz/${quizE.code}/paper`, null, jE.token))
  check('D5 试卷不含排行榜字段', !paperStr.includes('"ranking"'), '')

  } finally {
    // ---------- 清理本场测试数据（断言失败同样执行；只动本场创建的赛与用户） ----------
    let dq = 0, du = 0
    for (const code of createdQuizzes) {
      try {
        let r = await j('DELETE', `/api/admin/quiz/${code}`, null, at)
        if (r.code !== 0) { // RUNNING 等状态先结束再删
          await j('POST', `/api/admin/quiz/${code}/end`, {}, at)
          r = await j('DELETE', `/api/admin/quiz/${code}`, null, at)
        }
        if (r.code === 0) dq++
      } catch {}
    }
    try {
      const users = (await j('GET', '/api/admin/users', null, at)).data || []
      const byName = new Map(users.map(u => [u.username, u.id]))
      for (const name of createdUsers) {
        const id = byName.get(name)
        if (id && (await j('DELETE', `/api/admin/users/${id}`, null, at)).code === 0) du++ // 级联清 participants/answers
      }
    } catch {}
    console.log(`🧹 已清理测试数据：比赛 ${dq}/${createdQuizzes.length} 场、用户 ${du}/${createdUsers.length} 个`)
  }
  console.log(`\n${fail === 0 ? 'ALL PASS' : 'HAS FAILURES'}: ${pass} passed, ${fail} failed`)
  process.exit(fail === 0 ? 0 : 1)
})().catch(e => { console.error('FAIL:', e.message); process.exit(1) })
