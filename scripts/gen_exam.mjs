// 生成计分规则验证赛：
//   必答题（required=true，答错不扣分）  总分 40 = 8 单选×2 + 6 多选×4
//   抢答题（required=false，答错扣本题分值）总分 60 = 6 单选×3 + 7 多选×6
// 用法：node scripts/gen_exam.mjs
import { execSync as sh } from 'node:child_process'
import { readFileSync } from 'node:fs'
const env = k => process.env[k] || (readFileSync('.env', 'utf8').match(new RegExp('^' + k + '=(.*)$', 'm')) || [])[1] || ''

const B = process.env.BASE_URL || 'http://127.0.0.1:18080'
const ADMIN_PASS = env('ADMIN_PASS')
const MYSQL_PASS = env('MYSQL_ROOT_PASSWORD')
const REDIS_PASS = env('REDIS_PASSWORD')

const j = async (method, path, body, token) => {
  const r = await fetch(B + path, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
    body: body === null ? undefined : JSON.stringify(body),
  })
  return r.json()
}

// ---------- 1. 清库（保留用户表不动？不——完整清库，与 seed 一致） ----------
sh(`docker exec kaoshi-mysql mysql -uroot -p${MYSQL_PASS} kaoshi -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"`, { stdio: ['ignore', 'pipe', 'pipe'] })
sh(`docker exec kaoshi-redis redis-cli -a ${REDIS_PASS} FLUSHDB`, { stdio: ['ignore', 'pipe', 'pipe'] })
try { sh('docker restart kaoshi-server', { stdio: ['ignore', 'pipe', 'pipe'] }) } catch {}
console.log('🗑  已清空全部数据')

let at = null
for (let i = 0; i < 30 && !at; i++) {
  try { at = (await j('POST', '/api/admin/login', { username: 'admin', password: ADMIN_PASS })).data.token } catch { await new Promise(r => setTimeout(r, 1000)) }
}
if (!at) throw new Error('admin 登录失败')

// ---------- 2. 选手账号 ----------
for (const u of [
  { username: 'player1', password: 'player12345', nickname: '选手一号' },
  { username: 'player2', password: 'player12345', nickname: '选手二号' },
  { username: 'player3', password: 'player12345', nickname: '选手三号' },
]) {
  const r = await j('POST', '/api/admin/users', u, at)
  if (r.code !== 0) throw new Error('建号失败: ' + r.msg)
}

// ---------- 3. 题库 ----------
const SINGLE = [ // [题干, 正确答案label, 4 个选项内容]
  ['中国的首都是哪座城市？', 1, ['上海', '北京', '广州', '深圳']],
  ['一年有多少天（非闰年）？', 2, ['364 天', '360 天', '365 天', '366 天']],
  ['《静夜思》的作者是谁？', 3, ['杜甫', '白居易', '王维', '李白']],
  ['光在真空中的速度约为每秒多少公里？', 0, ['30 万公里', '3 万公里', '300 公里', '3 公里']],
  ['世界上国土面积最大的国家是？', 0, ['俄罗斯', '加拿大', '中国', '美国']],
  ['水的化学式是？', 2, ['HO', 'H₂O₂', 'H₂O', 'OH']],
  ['奥运会每几年举办一届？', 1, ['3 年', '4 年', '5 年', '2 年']],
  ['《西游记》中孙悟空的金箍棒又叫什么？', 2, ['定海神铁', '如意神针', '如意金箍棒', '降魔宝杖']],
  ['地球自转一周约需要多少小时？', 1, ['12 小时', '24 小时', '36 小时', '48 小时']],
  ['"杂交水稻之父"是谁？', 3, ['钱学森', '竺可桢', '李四光', '袁隆平']],
  ['二进制数 1010 等于十进制的多少？', 1, ['8', '10', '12', '14']],
  ['人体最大的器官是？', 0, ['皮肤', '肝脏', '大脑', '心脏']],
  ['声音在哪种介质中传播最快？', 2, ['空气', '水', '钢铁', '真空']],
  ['珠穆朗玛峰位于哪座山脉？', 0, ['喜马拉雅山脉', '昆仑山脉', '天山山脉', '阿尔卑斯山脉']],
]
const MULTI = [ // [题干, 正确答案labels, 4-5 个选项]
  ['以下哪些是编程语言？', [0, 2, 3], ['Python', 'HTTP', 'C++', 'Go']],
  ['以下哪些属于中国的四大发明？', [0, 2, 3], ['造纸术', '地动仪', '印刷术', '火药']],
  ['以下哪些是哺乳动物？', [0, 3], ['海豚', '鲨鱼', '鳄鱼', '蝙蝠']],
  ['以下哪些是中国的直辖市？', [0, 2, 3], ['北京', '成都', '上海', '重庆']],
  ['以下哪些属于可再生能源？', [0, 2, 3], ['风能', '煤炭', '太阳能', '水能']],
  ['以下哪些是 HTTP 请求方法？', [0, 1, 3], ['GET', 'POST', 'FTP', 'DELETE']],
  ['以下哪些颜色属于三原色（光的三原色）？', [0, 2, 3], ['红', '黄', '绿', '蓝']],
  ['以下哪些国家位于亚洲？', [0, 2, 3], ['日本', '埃及', '印度', '韩国']],
  ['以下哪些是维生素？', [0, 1, 3], ['维生素 A', '维生素 C', '蛋白质', '维生素 D']],
  ['以下哪些是浏览器？', [0, 2, 3], ['Chrome', 'MySQL', 'Firefox', 'Safari']],
  ['以下哪些属于唐诗三百首收录的诗人体裁常见的题材？', [0, 1, 2], ['边塞', '田园', '送别', 'SQL 注入']],
  ['以下哪些是中国的传统节日？', [0, 2, 3], ['春节', '圣诞节', '中秋节', '端午节']],
  ['以下哪些单位属于国际单位制基本单位？', [0, 2, 3], ['米', '升', '千克', '秒']],
]
const L = 'ABCDEFGH'.split('')

// ---------- 4. 建赛 ----------
const quiz = (await j('POST', '/api/admin/quiz', {
  title: '计分规则验证赛', mode: 'normal',
  description: '必答题 40 分（单选 2 分/题、多选 4 分/题，答错不扣分）；抢答题 60 分（单选 3 分/题、多选 6 分/题，答错扣本题分值）',
  per_question_time: 30, rush_enabled: true, rush_time: 10, rush_answer_time: 20,
  rush_winner_count: 1, rush_bonus_score: 0, rush_wrong_score: 1,
  show_answer: true, show_analysis: true, show_ranking: true,
}, at)).data
if (!quiz?.id) throw new Error('建赛失败')

let sort = 0
const mk = async (type, pool, idx, score, required, timeLimit) => {
  const [content, correct, options] = pool[idx]
  const correctArr = Array.isArray(correct) ? correct : [correct]
  sort++
  const r = await j('POST', `/api/admin/quiz/${quiz.id}/questions`, {
    type, content,
    answer: correctArr.map(i => L[i]).sort().join(''),
    analysis: required ? '必答题：答对得分，答错不扣分。' : '抢答题：答对得分，答错扣本题分值。',
    score, required, time_limit: timeLimit, sort,
    options: options.map((c, i) => ({ label: L[i], content: c })),
  }, at)
  if (r.code !== 0) throw new Error('建题失败 #' + sort + ': ' + r.msg)
}

// 必答：8 单选×2 + 6 多选×4 = 40 分（答错不扣分）
for (let i = 0; i < 8; i++) await mk('single', SINGLE, i, 2, true, 30)
for (let i = 0; i < 6; i++) await mk('multiple', MULTI, i, 4, true, 40)
// 抢答：6 单选×3 + 7 多选×6 = 60 分（答错扣本题分值）
for (let i = 8; i < 14; i++) await mk('single', SINGLE, i, 3, false, 30)
for (let i = 6; i < 13; i++) await mk('multiple', MULTI, i, 6, false, 40)

console.log(`✅ 已生成「计分规则验证赛」#${quiz.id}（27 题，共 100 分）`)
console.log('   必答题 40 分：8 单选×2 + 6 多选×4（答错不扣分）')
console.log('   抢答题 60 分：6 单选×3 + 7 多选×6（答错扣本题分值；rush_wrong_score=1 表示开启扣分）')
console.log('   选手：player1-3 / player12345')
console.log(`   流程：控制台 ▶开始 → 必答题直接作答；到抢答题时点 ⚡开始抢答 → 抢到者作答（答错倒扣）`)
