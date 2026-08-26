// 生成计分规则验证赛：
//   必答题（required=true，答错不扣分）  总分 40 = 8 单选×2 + 6 多选×4
//   抢答题（required=false）总分 60 = 10 单选×3 + 5 多选×6
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
  { username: 'zhangwei', password: 'zhangwei12345', nickname: '张伟' },
  { username: 'liuyang', password: 'liuyang12345', nickname: '刘洋' },
  { username: 'chenjing', password: 'chenjing12345', nickname: '陈静' },
  { username: 'wangfang', password: 'wangfang12345', nickname: '王芳' },
  { username: 'zhaolei', password: 'zhaolei12345', nickname: '赵磊' },
]) {
  const r = await j('POST', '/api/admin/users', u, at)
  if (r.code !== 0) throw new Error('建号失败: ' + r.msg)
}

// ---------- 3. 题库 ----------
const SINGLE = [ // [题干, 正确答案下标, 选项（中英混排）]
  ['HTTPS 默认使用的端口号是？', 2, ['21 FTP', '80 HTTP', '443 HTTPS', '8080 代理端口']],
  ['SQL 注入攻击的主要目标是？', 1, ['Web 服务器 CPU', '数据库 Database', '用户浏览器缓存', 'DNS 服务器']],
  ['跨站脚本攻击的英文缩写是？', 0, ['XSS', 'CSRF', 'SSRF', 'XXE']],
  ['对称加密算法典型代表是？', 3, ['RSA', 'ECC', 'DSA', 'AES']],
  ['防火墙 Firewall 的主要作用是？', 1, ['加密所有数据', '控制网络访问流量', '查杀病毒', '备份数据']],
  ['“撞库”攻击利用的是？', 2, ['系统漏洞', '邮件附件', '重复使用的密码', 'WiFi 信号']],
  ['SSL/TLS 协议工作在 OSI 模型的哪一层？', 1, ['网络层 Network', '传输层与会话层之间', '物理层', '应用层之上最高层']],
  ['MD5 算法目前不安全的主要原因是？', 0, ['存在碰撞 Collision', '速度太慢', '密钥太长', '不支持中文']],
  ['钓鱼网站 Phishing 最常见的传播途径是？', 3, ['U 盘拷贝', '蓝牙传输', '系统更新', '伪装链接/邮件']],
  ['WAF 的全称是？', 2, ['Wide Area Firewall', 'Web Attack Filter', 'Web Application Firewall', 'Wireless Access Forward']],
  ['暴力破解 Brute Force 的有效防御手段是？', 1, ['加长用户名', '限制尝试次数+验证码', '更换浏览器', '定期重启服务器']],
  ['最小权限原则 Least Privilege 指？', 2, ['只给管理员权限', '权限平均分配', '仅授予完成任务所需权限', '所有人只读']],
  ['社会工程学攻击 Social Engineering 的核心是？', 0, ['利用人的心理弱点', '破解加密算法', '伪造 IP 包', '劫持 DNS']],
  ['零日漏洞 Zero-day 指？', 3, ['元旦发现的漏洞', '影响为零的漏洞', '已修复的漏洞', '厂商尚无补丁的漏洞']],
  ['ARP 欺骗攻击主要发生在？', 1, ['应用层', '局域网链路层', '传输层', '物理电缆']],
  ['DNS 劫持导致的直接后果是？', 2, ['网速变慢', '电脑死机', '访问被引导到假网站', '硬盘被格式化']],
  ['证书 Certificate 过期后浏览器会？', 0, ['提示风险/拒绝访问', '自动续期', '无任何变化', '加速访问']],
  ['“蜜罐” Honeypot 的用途是？', 1, ['存储蜂蜜数据', '诱捕分析攻击者', '备份数据库', '加速网络']],
  ['VPN 的核心安全作用是？', 2, ['免费上网', '隐藏 IP 用于攻击', '建立加密隧道', '替代防火墙']],
  ['密码学中“盐” Salt 的作用是？', 3, ['增加密码长度', '加快哈希速度', '压缩存储空间', '抵御彩虹表破解']],
]
const MULTI = [ // [题干, 正确答案下标数组, 选项（中英混排）]
  ['以下哪些属于常见的 Web 安全漏洞？', [0, 1, 3], ['SQL 注入 Injection', 'XSS 跨站脚本', 'TCP 三次握手', 'CSRF 伪造请求']],
  ['以下哪些是强密码的特征？', [0, 2, 3], ['长度 ≥12 位', '包含生日信息', '大小写+数字+符号混合', '不含常见单词']],
  ['以下哪些属于对称加密算法？', [0, 2], ['DES', 'RSA', 'SM4 国密', 'ECC']],
  ['以下哪些是 HTTPS 的作用？', [0, 1, 2], ['加密传输内容', '验证服务器身份', '防止流量被窃听篡改', '自动查杀病毒']],
  ['以下哪些行为存在安全隐患？', [0, 2, 3], ['公共 WiFi 下登录网银', '使用官方应用商店下载 App', '多个网站共用同一密码', '点击不明短信链接']],
  ['以下哪些是常见的网络扫描工具？', [0, 2, 3], ['Nmap', 'Photoshop', 'Wireshark', 'Burp Suite']],
  ['DDoS 攻击的特征包括？', [0, 1, 3], ['大量傀儡机 Zombie', '耗尽目标带宽资源', '窃取数据库内容', '服务不可用 DoS 升级版']],
  ['以下哪些属于个人信息保护措施？', [0, 1, 3], ['开启双因素认证 2FA', '定期更换密码', '朋友圈晒身份证照片', '不在陌生网站填手机号']],
  ['以下哪些是安全传输/认证协议？', [0, 1, 3], ['TLS 1.3', 'SSH', 'HTTP 明文', 'Kerberos']],
  ['以下哪些属于恶意软件类型？', [0, 1, 3], ['木马 Trojan', '蠕虫 Worm', '杀毒软件 Antivirus', '勒索软件 Ransomware']],
  ['日志审计的价值包括？', [0, 1, 2], ['追溯攻击来源', '发现异常行为', '满足合规要求', '提升 WiFi 速度']],
  ['以下哪些是 XSS 的防御手段？', [0, 2, 3], ['输出编码 Output Encoding', '关闭防火墙', 'CSP 内容安全策略', 'HttpOnly Cookie']],
  ['以下哪些属于网络安全的 CIA 三要素？', [0, 1, 3], ['机密性 Confidentiality', '完整性 Integrity', '高性能 Availability 高并发', '可用性 Availability']],
]
const L = 'ABCDEFGH'.split('')

// ---------- 4. 建赛 ----------
const quiz = (await j('POST', '/api/admin/quiz', {
  title: '计分规则验证赛', mode: 'normal',
  description: '必答题 40 分（单选 2 分/题、多选 4 分/题，答错 0 分）；抢答题 60 分（单选 3 分/题、多选 6 分/题，答错扣对应分值：单选-3、多选-6）',
  per_question_time: 30, rush_enabled: true, rush_time: 10, rush_answer_time: 20,
  rush_winner_count: 1,
  // 按题型计分（判分时覆盖题目分值）
  req_score_single: 2, req_score_multiple: 4, req_score_judge: 3,
  rush_score_single: 3, rush_score_multiple: 6, rush_score_judge: 3,
  rush_deduct_single: 3, rush_deduct_multiple: 6, rush_deduct_judge: 3,
  rush_deduct_single: 3, rush_deduct_multiple: 6, rush_deduct_judge: 3,
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
    analysis: '答对得本题分值；抢答题答错扣对应分值。',
    score, required, time_limit: timeLimit, sort,
    options: options.map((c, i) => ({ label: L[i], content: c })),
  }, at)
  if (r.code !== 0) throw new Error('建题失败 #' + sort + ': ' + r.msg)
}

// 必答 15 题：10 单选×2 + 5 多选×4 = 40 分（答错不扣分）
for (let i = 0; i < 10; i++) await mk('single', SINGLE, i, 2, true, 30)
for (let i = 0; i < 5; i++) await mk('multiple', MULTI, i, 4, true, 40)
// 抢答 15 题：10 单选×3 + 5 多选×6 = 60 分（答错各扣对应分值：单选3、多选6）
for (let i = 10; i < 20; i++) await mk('single', SINGLE, i, 3, false, 30)
for (let i = 5; i < 10; i++) await mk('multiple', MULTI, i, 6, false, 40)

console.log(`✅ 已生成「计分规则验证赛」#${quiz.id}（30 题，共 100 分）`)
console.log('   必答题 15 题 40 分：10 单选×2 + 5 多选×4（答错 0 分）')
console.log('   抢答题 15 题 60 分：10 单选×3 + 5 多选×6（答错扣对应分值：单选-3、多选-6）')
console.log('   选手（用户名=姓名拼音，密码=用户名+12345）：')
console.log('     zhangwei/张伟  liuyang/刘洋  chenjing/陈静  wangfang/王芳  zhaolei/赵磊')
console.log(`   流程：控制台 ▶开始 → 必答题直接作答；到抢答题时点 ⚡开始抢答 → 抢到者作答（答错倒扣）`)
