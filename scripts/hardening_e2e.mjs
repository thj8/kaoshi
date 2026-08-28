// 阶段8 加固 E2E：鉴权越权 / 防重复计分 / 答案泄露 / 100并发抢答 / 断线重连恢复 / 考试模式（自由切题）
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
  const createdQuizzes = [], createdUsers = [] // 本场创建的资源，结尾统一清理
  let at = null
  try {
  at = (await j('POST', '/api/admin/login', { username: 'admin', password: env('ADMIN_PASS') })).data.token
  const mkU = async u => { // 注册已下线：admin 建号 + 登录
    const NAMES = ['陈晨', '刘洋', '赵磊', '孙悦', '周杰', '吴倩', '郑浩', '王梅']
    await j('POST', '/api/admin/users', { username: u, password: 'test-pass-1234', nickname: NAMES[uN++ % NAMES.length] }, at)
    createdUsers.push(u)
    return (await j('POST', '/api/auth/login', { username: u, password: 'test-pass-1234' })).data
  }
  let uN = 0
  const sfx = Date.now() % 100000

  // ---------- 1. 普通 quiz：鉴权 / 防重复 / 答案泄露 ----------
  const quizA = (await j('POST', '/api/admin/quiz', { title: 's8-secA', mode: 'normal', per_question_time: 60, show_answer: true }, at)).data
  createdQuizzes.push(quizA.code)
  const quizB = (await j('POST', '/api/admin/quiz', { title: 's8-secB', mode: 'normal', per_question_time: 60 }, at)).data
  createdQuizzes.push(quizB.code)
  const qA = (await j('POST', `/api/admin/quiz/${quizA.code}/questions`, { type: 'single', content: 'A?', answer: 'B', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  await j('POST', `/api/admin/quiz/${quizB.code}/questions`, { type: 'single', content: 'B?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)

  const ua = await mkU(`s8u${sfx}`)
  const ja = (await j('POST', '/api/join', { quiz_id: quizA.code }, ua.token)).data
  const jb = (await j('POST', '/api/join', { quiz_id: quizB.code }, ua.token)).data

  // 1a. quizB 的 token 不能操作 quizA 的题
  await j('POST', `/api/admin/quiz/${quizA.code}/start`, {}, at); await sleep(400)
  const cross = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, jb.token)
  check('越权：跨 quiz token 提交被拒', cross.code !== 0, `code=${cross.code} msg=${cross.msg}`)

  // 1b. 未参加 quizA 的裸 token 也不能
  const stranger = await mkU(`s8x${sfx}`)
  const noJoin = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, stranger.token)
  check('越权：未参加者提交被拒', noJoin.code !== 0, `code=${noJoin.code}`)

  // 1c. 重复提交只计一次分
  const r1 = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, ja.token)
  const r2 = await j('POST', `/api/question/${qA.id}/answer`, { answer: 'B', duration: 100 }, ja.token)
  const res1 = (await j('GET', `/api/quiz/${quizA.code}/result`, null, ja.token)).data
  check('防重复：二次提交被拒或不再加分', res1.score === 10, `first.code=${r1.code} second.code=${r2.code} score=${res1.score}`)

  // 1d. 答案不下发：题目相关公开接口均无 answer/analysis 字段
  const cur = JSON.stringify(await j('GET', `/api/quiz/${quizA.code}/current-question`, null, ja.token))
  const info = JSON.stringify(await j('GET', `/api/quiz/${quizA.code}`, null, ja.token))
  check('答案不下发：current-question/info 无 answer/analysis', !/"answer":"[AB]"|"analysis":"/.test(cur + info), '')
  await j('POST', `/api/admin/quiz/${quizA.code}/end`, {}, at)

  // ---------- 2. 100 并发抢答唯一性 ----------
  const rq = (await j('POST', '/api/admin/quiz', { title: 's8-rush100', mode: 'rush', rush_winner_count: 1, rush_time: 15, rush_answer_time: 20, rush_bonus_score: 5, show_ranking: true, rush_countdown: 0 }, at)).data
  createdQuizzes.push(rq.code)
  const qr = (await j('POST', `/api/admin/quiz/${rq.code}/questions`, { type: 'single', content: 'R?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const N = 100
  const tokens = []
  for (let i = 0; i < N; i++) {
    const u = await mkU(`s8r${sfx}_${i}`)
    tokens.push((await j('POST', '/api/join', { quiz_id: rq.code }, u.token)).data.token)
  }
  await j('POST', `/api/admin/quiz/${rq.code}/start`, {}, at)
  await j('POST', `/api/admin/quiz/${rq.code}/rush/start`, {}, at)
  await sleep(400)
  const results = await Promise.allSettled(tokens.map(t => j('POST', `/api/question/${qr.id}/rush`, {}, t)))
  const winners = results.filter(r => r.status === 'fulfilled' && r.value.code === 0 && r.value.data?.rank === 1)
  const losers = results.filter(r => r.status === 'fulfilled' && r.value.code === 0 && r.value.data?.rank > 1)
  check(`100并发抢答：rank=1 唯一`, winners.length === 1, `winners=${winners.length} losers=${losers.length} errors=${results.length - winners.length - losers.length}`)
  await j('POST', `/api/admin/quiz/${rq.code}/rush/end`, {}, at)

  // ---------- 2b. 强制收卷后实时统计：未答占位行不计入已答/正确/错误 ----------
  const quizS = (await j('POST', '/api/admin/quiz', { title: 's8-stats', mode: 'normal', per_question_time: 60, show_answer: true }, at)).data
  createdQuizzes.push(quizS.code)
  const qS = (await j('POST', `/api/admin/quiz/${quizS.code}/questions`, { type: 'single', content: 'S?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const us1 = await mkU(`s8s1${sfx}`), us2 = await mkU(`s8s2${sfx}`), us3 = await mkU(`s8s3${sfx}`)
  const js1 = (await j('POST', '/api/join', { quiz_id: quizS.code }, us1.token)).data
  const js2 = (await j('POST', '/api/join', { quiz_id: quizS.code }, us2.token)).data
  const js3 = (await j('POST', '/api/join', { quiz_id: quizS.code }, us3.token)).data
  await j('POST', `/api/admin/quiz/${quizS.code}/start`, {}, at); await sleep(300)
  await j('POST', `/api/question/${qS.id}/answer`, { answer: 'A', duration: 100 }, js1.token) // 1 人答对
  await j('POST', `/api/question/${qS.id}/answer`, { answer: 'B', duration: 100 }, js2.token) // 1 人答错
  // us3 超时不答 → 强制收卷补未答占位行
  await j("POST", `/api/admin/quiz/${quizS.code}/reveal`, {}, at); await sleep(300)
  const stS = (await j('GET', `/api/admin/quiz/${quizS.code}/statistics`, null, at)).data
  const qSt = stS.questions?.find(x => x.question_id === qS.id)
  check('实时统计：已答不含未答占位(2/3)', qSt && qSt.answered === 2, `answered=${qSt?.answered}`)
  check('实时统计：正确/错误不含未答(1/1)', qSt && qSt.correct === 1 && qSt.wrong === 1, `correct=${qSt?.correct} wrong=${qSt?.wrong}`)
  check('实时统计：正确率分母为真实作答(50%)', Math.round(qSt.correct_rate) === 50, `rate=${qSt?.correct_rate}`)
  const rkS = stS.ranking || []
  const r1S = rkS.find(r => r.correct === 1), r2S = rkS.find(r => r.wrong === 1 && r.correct === 0)
  check('S3 答对按题目分值(+10)、答错 0 分', r1S?.score === 10 && r2S?.score === 0, `对=${r1S?.score} 错=${r2S?.score}`)

  // ---------- 2c. 成绩总分与排行榜（T 系列） ----------
  const quizT = (await j('POST', '/api/admin/quiz', { title: 's8-total', mode: 'normal', per_question_time: 60, show_answer: true, show_ranking: true }, at)).data
  createdQuizzes.push(quizT.code)
  await j('POST', `/api/admin/quiz/${quizT.code}/questions`, { type: 'single', content: 'T1?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)
  await j('POST', `/api/admin/quiz/${quizT.code}/questions`, { type: 'single', content: 'T2?', answer: 'B', score: 10, required: true, sort: 2, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)
  const ut1 = await mkU(`s8t1${sfx}`), ut2 = await mkU(`s8t2${sfx}`), ut3 = await mkU(`s8t3${sfx}`)
  const jt1 = (await j('POST', '/api/join', { quiz_id: quizT.code }, ut1.token)).data
  const jt2 = (await j('POST', '/api/join', { quiz_id: quizT.code }, ut2.token)).data
  await j('POST', '/api/join', { quiz_id: quizT.code }, ut3.token) // 只加入不答题
  const qsT = (await j('GET', `/api/admin/quiz/${quizT.code}/questions`, null, at)).data
  await j('POST', `/api/admin/quiz/${quizT.code}/start`, {}, at); await sleep(400)
  await j('POST', `/api/question/${qsT[0].id}/answer`, { answer: 'A', duration: 100 }, jt1.token) // u1 对
  await j('POST', `/api/question/${qsT[0].id}/answer`, { answer: 'B', duration: 100 }, jt2.token) // u2 错
  await j('POST', `/api/admin/quiz/${quizT.code}/next`, {}, at); await sleep(400)
  const t1q2 = await j('POST', `/api/question/${qsT[1].id}/answer`, { answer: 'B', duration: 100 }, jt1.token)
  check('T1 即时 total_score 跨题累计', t1q2.data?.score === 10 && t1q2.data?.total_score === 20, `score=${t1q2.data?.score} total=${t1q2.data?.total_score}`)
  // 过题防护：主持人已切到第 2 题，对第 1 题补交/改答案必须被拒（服务端仅接受当前题）
  const oldRe = await j('POST', `/api/question/${qsT[0].id}/answer`, { answer: 'A', duration: 100 }, jt1.token)
  check('T6 过题防护：切题后提交已过去的题目被拒', oldRe.code !== 0 && /当前题目不匹配/.test(oldRe.msg || ''), `code=${oldRe.code} msg=${oldRe.msg}`)
  const oldMod = await j('POST', `/api/question/${qsT[0].id}/answer`, { answer: 'A', duration: 100 }, jt2.token)
  check('T7 过题防护：切题后更改已答题目答案被拒', oldMod.code !== 0 && /当前题目不匹配/.test(oldMod.msg || ''), `code=${oldMod.code} msg=${oldMod.msg}`)
  await j('POST', `/api/question/${qsT[1].id}/answer`, { answer: 'B', duration: 100 }, jt2.token) // u2 对
  await sleep(300)
  const stT = (await j('GET', `/api/admin/quiz/${quizT.code}/statistics`, null, at)).data
  const rk = stT.ranking || []
  const rk1 = rk[0], rk2 = rk[1]
  check('T2 排行榜按分数降序', rk.length >= 2 && rk1.score > rk2.score && rk1.rank === 1 && rk2.rank === 2, `r1=${rk1?.rank}/${rk1?.score} r2=${rk2?.rank}/${rk2?.score}`)
  check('T3 总分=各题得分之和(20/10/0)', rk1.score === 20 && rk2.score === 10 && rk.every(r => [20, 10, 0].includes(r.score)), JSON.stringify(rk.map(r => r.score)))
  check('T4 未答题者 0 分在榜', rk.some(r => r.score === 0), '')
  const resT = (await j('GET', `/api/quiz/${quizT.code}/result`, null, jt1.token)).data
  check('T5 成绩单总分=累计分', resT.total_score === 20 || resT.score === 20, JSON.stringify(resT).slice(0, 120))

  // ---------- 3. 断线重连恢复 ----------
  const quizC = (await j('POST', '/api/admin/quiz', { title: 's8-reconnect', mode: 'normal', per_question_time: 120 }, at)).data
  createdQuizzes.push(quizC.code)
  const qC = (await j('POST', `/api/admin/quiz/${quizC.code}/questions`, { type: 'single', content: 'C?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const uc = await mkU(`s8c${sfx}`)
  const jc = (await j('POST', '/api/join', { quiz_id: quizC.code }, uc.token)).data
  let synced = null
  const cdMsgs = []
  const ws1 = await connect(jc.token, m => { if (m.event === 'sync') synced = m.data })
  await j('POST', `/api/admin/quiz/${quizC.code}/start`, {}, at); await sleep(500)
  ws1.close() // 断线
  await sleep(300)
  const ws2 = await connect(jc.token, m => { if (m.event === 'sync') synced = m.data; if (m.event === 'question:countdown') cdMsgs.push(m.data) })
  await sleep(2500) // 至少覆盖两次每秒倒计时广播
  check('CD 倒计时每秒广播：含剩余秒与截止时间', cdMsgs.length >= 2 && cdMsgs.every(d => Number.isFinite(d.remain_sec) && d.remain_sec > 0 && d.deadline_at > 0 && d.question_id === qC.id), `n=${cdMsgs.length} last=${JSON.stringify(cdMsgs.at(-1))}`)
  await sleep(500)
  check('重连恢复：sync 带回状态与当前题', !!synced && synced.status === 'ANSWERING' && synced.question?.id === qC.id, `status=${synced?.status} q=${synced?.question?.id}`)
  const sub = await j('POST', `/api/question/${qC.id}/answer`, { answer: 'A', duration: 300 }, jc.token)
  check('重连后仍可作答', sub.code === 0, `code=${sub.code}`)
  ws2.close()
  await j('POST', `/api/admin/quiz/${quizC.code}/end`, {}, at)

  // ---------- 4. 考试模式（自由切题） ----------
  const badMode = await j('POST', '/api/admin/quiz', { title: 's8-badmode', mode: 'free', per_question_time: 30 }, at)
  check('E1 非法 mode 被拒(oneof)', badMode.code !== 0, `code=${badMode.code}`)
  const eq = (await j('POST', '/api/admin/quiz', { title: 's8-exam', mode: 'exam', total_time: 120, show_answer: true, show_ranking: true }, at)).data
  createdQuizzes.push(eq.code)
  check('E2 考试模式创建 mode=exam', eq.mode === 'exam', `mode=${eq.mode}`)
  await j('POST', `/api/admin/quiz/${eq.code}/questions`, { type: 'single', content: 'E-单选?', answer: 'B', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)
  await j('POST', `/api/admin/quiz/${eq.code}/questions`, { type: 'judge', content: 'E-判断?', answer: 'A', score: 10, required: true, sort: 2, options: [{ label: 'A', content: '对' }, { label: 'B', content: '错' }] }, at)
  await j('POST', `/api/admin/quiz/${eq.code}/questions`, { type: 'multiple', content: 'E-多选?', answer: 'AC', score: 10, required: true, sort: 3, options: [{ label: 'A', content: '2' }, { label: 'B', content: '3' }, { label: 'C', content: '4' }] }, at)
  const ue1 = await mkU(`s8e1${sfx}`), ue2 = await mkU(`s8e2${sfx}`), ue3 = await mkU(`s8e3${sfx}`)
  const je1 = (await j('POST', '/api/join', { quiz_id: eq.code }, ue1.token)).data
  const je2 = (await j('POST', '/api/join', { quiz_id: eq.code }, ue2.token)).data
  const je3 = (await j('POST', '/api/join', { quiz_id: eq.code }, ue3.token)).data

  const pWait = await j('GET', `/api/quiz/${eq.code}/paper`, null, je1.token)
  check('E3 未开始(WAITING)不下发题目但含题数', pWait.code === 0 && pWait.data.status === 'WAITING' && pWait.data.questions.length === 0 && pWait.data.question_count === 3, `n=${pWait.data?.questions?.length} cnt=${pWait.data?.question_count}`)
  await j('POST', `/api/admin/quiz/${eq.code}/start`, {}, at); await sleep(400)
  const pp1 = (await j('GET', `/api/quiz/${eq.code}/paper`, null, je1.token)).data
  const [pq1, pq2, pq3] = pp1.questions
  check('E4 开考全卷下发：3题+截止时间', pp1.status === 'RUNNING' && pp1.questions.length === 3 && pp1.deadline_at > 0, `n=${pp1.questions.length} dl=${pp1.deadline_at}`)
  check('E5 试卷不含 answer/analysis', !/"answer":|"analysis":/.test(JSON.stringify(pp1)), '')

  // 自由作答：乱序多选 / 非法选项 / 清空草稿 / 修改答案（不按出题顺序）
  const s3 = await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq3.id, answer: 'CA', duration: 100 }, je1.token)
  check('E6 多选乱序归一化(CA→AC)', s3.code === 0, `msg=${s3.msg}`)
  const sBad = await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq1.id, answer: 'XZ' }, je1.token)
  check('E7 非法选项被拒', sBad.code !== 0, `msg=${sBad.msg}`)
  const sClr = await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq3.id, answer: '', duration: 100 }, je1.token)
  const ppClr = (await j('GET', `/api/quiz/${eq.code}/paper`, null, je1.token)).data
  check('E8 空答案清除草稿(全取消=未答)', sClr.code === 0 && ppClr.questions.find(q => q.id === pq3.id).my_answer === null, `msg=${sClr.msg}`)
  await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq1.id, answer: 'A' }, je1.token)
  await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq1.id, answer: 'B' }, je1.token)
  await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq3.id, answer: 'AC' }, je1.token)
  const ppMod = (await j('GET', `/api/quiz/${eq.code}/paper`, null, je1.token)).data
  check('E9 交卷前可改答案(A→B 生效)', ppMod.questions.find(q => q.id === pq1.id).my_answer === 'B')

  // ue2：q1对 + q2错；ue3：并发首存同一题
  await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq1.id, answer: 'B' }, je2.token)
  await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq2.id, answer: 'B' }, je2.token)
  const race = await Promise.all([
    j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq3.id, answer: 'A' }, je3.token),
    j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq3.id, answer: 'AC' }, je3.token),
  ])
  const pp3 = (await j('GET', `/api/quiz/${eq.code}/paper`, null, je3.token)).data
  const u3ans = pp3.questions.find(q => q.id === pq3.id).my_answer
  check('E10 并发首存同一题：均成功且仅一条记录', race.every(r => r.code === 0) && ['A', 'AC'].includes(u3ans), `final=${u3ans}`)

  const liveAns = await j('POST', `/api/question/${pq1.id}/answer`, { answer: 'B', duration: 100 }, je1.token)
  const liveNext = await j('POST', `/api/admin/quiz/${eq.code}/next`, {}, at)
  const liveRush = await j('POST', `/api/admin/quiz/${eq.code}/rush/start`, {}, at)
  const livePause = await j('POST', `/api/admin/quiz/${eq.code}/pause`, {}, at)
  check('E11 考试模式屏蔽逐题/流程接口', liveAns.code !== 0 && liveNext.code !== 0 && liveRush.code !== 0 && livePause.code !== 0, `ans=${liveAns.msg} next=${liveNext.msg}`)

  const sub1 = (await j('POST', `/api/quiz/${eq.code}/paper/submit`, {}, je1.token)).data
  check('E12 交卷判分：2对20分、未答不算错', sub1.score === 20 && sub1.correct === 2 && sub1.wrong === 0 && sub1.answered === 2, JSON.stringify(sub1))
  const postSave = await j('POST', `/api/quiz/${eq.code}/paper/answer`, { question_id: pq2.id, answer: 'A' }, je1.token)
  check('E13 交卷后修改答案被拒', postSave.code !== 0, `msg=${postSave.msg}`)
  const resub = await j('POST', `/api/quiz/${eq.code}/paper/submit`, {}, je1.token)
  check('E14 重复交卷幂等(同分)', resub.code === 0 && resub.data.score === 20, `code=${resub.code}`)
  const sub2 = (await j('POST', `/api/quiz/${eq.code}/paper/submit`, {}, je2.token)).data
  check('E15 答错0分(10分,1对1错)', sub2.score === 10 && sub2.correct === 1 && sub2.wrong === 1, JSON.stringify(sub2))

  // 重连恢复：考试态 sync + 收卷广播（同一条连接）
  let esync = null, endRank = null
  const ews = await connect(je3.token, m => {
    if (m.event === 'sync') esync = m.data
    if (m.event === 'activity:end') endRank = m.data
  })
  await sleep(400)
  check('E16 重连恢复：sync 回考试态(RUNNING+deadline,无单题)', !!esync && esync.status === 'RUNNING' && esync.deadline_at > 0 && !esync.question, `st=${esync?.status} dl=${esync?.deadline_at}`)
  await j('POST', `/api/admin/quiz/${eq.code}/end`, {}, at); await sleep(500)
  ews.close()
  check('E17 activity:end 广播排行榜(3人降序)', Array.isArray(endRank?.ranking) && endRank.ranking.length === 3 && endRank.ranking[0].score === 20 && endRank.ranking[0].score >= endRank.ranking[1].score && endRank.ranking[1].score >= endRank.ranking[2].score, JSON.stringify(endRank?.ranking?.map(r => r.score)))
  const r3 = (await j('GET', `/api/quiz/${eq.code}/result`, null, je3.token)).data
  check('E18 统一收卷：未交卷者按已存答案计分', r3.finished === true && r3.score === (u3ans === 'AC' ? 10 : 0), `score=${r3.score} ans=${u3ans}`)
  const r1f = (await j('GET', `/api/quiz/${eq.code}/result`, null, je1.token)).data
  check('E19 结束重算不改动已交卷成绩', r1f.score === 20 && r1f.finished === true, `score=${r1f.score}`)

  // ---------- 4b. 考试到时自动收卷 ----------
  const eq2 = (await j('POST', '/api/admin/quiz', { title: 's8-exam-timer', mode: 'exam', total_time: 3, show_ranking: true }, at)).data
  createdQuizzes.push(eq2.code)
  const pq = (await j('POST', `/api/admin/quiz/${eq2.code}/questions`, { type: 'judge', content: 'E-自动收卷?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '是' }, { label: 'B', content: '否' }] }, at)).data
  const ue4 = await mkU(`s8e4${sfx}`)
  const je4 = (await j('POST', '/api/join', { quiz_id: eq2.code }, ue4.token)).data
  await j('POST', `/api/admin/quiz/${eq2.code}/start`, {}, at)
  await j('POST', `/api/quiz/${eq2.code}/paper/answer`, { question_id: pq.id, answer: 'A', duration: 100 }, je4.token)
  await sleep(4600)
  const r4 = (await j('GET', `/api/quiz/${eq2.code}/result`, null, je4.token)).data
  check('E20 到时自动收卷+重算', r4.finished === true && r4.score === 10, `score=${r4.score} finished=${r4.finished}`)
  const late = await j('POST', `/api/quiz/${eq2.code}/paper/answer`, { question_id: pq.id, answer: 'B' }, je4.token)
  check('E21 到时后保存被拒', late.code !== 0, `msg=${late.msg}`)

  // ---------- 4c. 考试实时排行榜（管理端 statistics）+ 排行榜仅管理员可见 ----------
  const eq3 = (await j('POST', '/api/admin/quiz', { title: 's8-exam-liverank', mode: 'exam', total_time: 300, show_ranking: true }, at)).data
  createdQuizzes.push(eq3.code)
  const lq1 = (await j('POST', `/api/admin/quiz/${eq3.code}/questions`, { type: 'single', content: 'E-实时榜1?', answer: 'A', score: 10, required: true, sort: 1, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const lq2 = (await j('POST', `/api/admin/quiz/${eq3.code}/questions`, { type: 'single', content: 'E-实时榜2?', answer: 'B', score: 10, required: true, sort: 2, options: [{ label: 'A', content: '1' }, { label: 'B', content: '2' }] }, at)).data
  const ue5 = await mkU(`s8e5${sfx}`), ue6 = await mkU(`s8e6${sfx}`)
  const jr5 = (await j('POST', '/api/join', { quiz_id: eq3.code }, ue5.token)).data
  const jr6 = (await j('POST', '/api/join', { quiz_id: eq3.code }, ue6.token)).data
  const t0c = Date.now()
  await j('POST', `/api/admin/quiz/${eq3.code}/start`, {}, at); await sleep(300)
  const id5 = ue5.user.id, id6 = ue6.user.id
  const lr = (s, id) => s.ranking.find(r => r.user_id === id)

  // 实时排名：未交卷也计分（服务端草稿判分聚合），排行榜数据走 /api/admin/quiz/:id/statistics
  await j('POST', `/api/quiz/${eq3.code}/paper/answer`, { question_id: lq1.id, answer: 'A', duration: 100 }, jr5.token)
  await sleep(30)
  await j('POST', `/api/quiz/${eq3.code}/paper/answer`, { question_id: lq1.id, answer: 'A', duration: 100 }, jr6.token)
  await sleep(60)
  let ls = (await j('GET', `/api/admin/quiz/${eq3.code}/statistics`, null, at)).data
  check('E22 实时排行榜：未交卷也显示得分(10/10)', lr(ls, id5)?.score === 10 && lr(ls, id6)?.score === 10, JSON.stringify(ls.ranking.map(r => [r.nickname, r.score])))
  // 考试未交卷时 result 必须拒绝：否则脚本可「逐题保存选项→拉 result 看对错数变化」试答猜题
  const rProbe = await j('GET', `/api/quiz/${eq3.code}/result`, null, jr5.token)
  check('E31 考试未交卷拉成绩被拒(防试答探测)', rProbe.code !== 0, `code=${rProbe.code} msg=${rProbe.msg}`)
  check('E23 实时排行榜：同分未交卷先到分者排前', lr(ls, id5)?.rank === 1 && lr(ls, id6)?.rank === 2, `r5=${lr(ls, id5)?.rank} r6=${lr(ls, id6)?.rank}`)
  check('E24 实时汇总：max=min=avg=10、逐题已答=2', ls.max_score === 10 && ls.min_score === 10 && Math.abs(ls.avg_score - 10) < 0.01 && ls.questions[0].answered === 2, `max=${ls.max_score} avg=${ls.avg_score} ans=${ls.questions[0].answered}`)
  await sleep(2000) // 拉开首答→交卷时间差，供 E30 用时断言

  // 同分交卷顺序：已交卷者排前；都已交卷按交卷时间早者在前
  await j('POST', `/api/quiz/${eq3.code}/paper/submit`, {}, jr6.token); await sleep(60)
  ls = (await j('GET', `/api/admin/quiz/${eq3.code}/statistics`, null, at)).data
  check('E25 同分：已交卷者排前、未交卷仍实时计分', lr(ls, id6)?.rank === 1 && lr(ls, id5)?.rank === 2 && lr(ls, id5)?.score === 10 && ls.finished === 1, JSON.stringify(ls.ranking.map(r => [r.nickname, r.rank, r.score])))
  check('E25b 交卷时间字段：已交卷>0、未交卷=0', lr(ls, id6)?.submitted_at > 0 && lr(ls, id5)?.submitted_at === 0, `sub6=${lr(ls, id6)?.submitted_at} sub5=${lr(ls, id5)?.submitted_at}`)
  await j('POST', `/api/quiz/${eq3.code}/paper/submit`, {}, jr5.token); await sleep(60)
  ls = (await j('GET', `/api/admin/quiz/${eq3.code}/statistics`, null, at)).data
  check('E26 同分都已交卷：交卷早者排前（含交卷时间）', lr(ls, id6)?.rank === 1 && lr(ls, id5)?.rank === 2 && ls.finished === 2 && lr(ls, id5)?.submitted_at > 0, JSON.stringify(ls.ranking.map(r => [r.nickname, r.rank, r.submitted_at])))

  // 排行榜权限：考试模式选手不可看排行榜；statistics 仅 admin
  const uRank = await j('GET', `/api/quiz/${eq3.code}/ranking`, null, jr5.token)
  check('E27 考试模式选手拉排行榜被拒(仅管理员)', uRank.code !== 0, `msg=${uRank.msg}`)
  const uStats = await j('GET', `/api/admin/quiz/${eq3.code}/statistics`, null, jr5.token)
  check('E28 用户 token 访问管理统计被拒', uStats.code !== 0, `code=${uStats.code}`)
  const noTok = await fetch(`${B}/api/admin/quiz/${eq3.code}/statistics`).then(r => r.status)
  check('E29 无 token 访问管理统计被拒(401)', noTok === 401, `http=${noTok}`)

  // 考试用时：墙钟口径（本人首答→交卷），不能逐题 duration 累加（窗口重叠会虚高）
  const wallSec = Math.round((Date.now() - t0c) / 1000)
  const r5res = (await j('GET', `/api/quiz/${eq3.code}/result`, null, jr5.token)).data
  check('E30 考试用时=首答→交卷墙钟（>0 且 ≤ 实际耗时+2s）', r5res.duration_sec > 0 && r5res.duration_sec <= wallSec + 2, `dur=${r5res.duration_sec}s wall=${wallSec}s`)

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
