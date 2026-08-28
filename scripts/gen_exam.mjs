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
sh(`docker exec kaoshi-mysql mysql -uroot -p${MYSQL_PASS} kaoshi -e "SET FOREIGN_KEY_CHECKS=0; TRUNCATE answers; TRUNCATE rush_records; TRUNCATE participants; TRUNCATE quiz_invitees; TRUNCATE question_options; TRUNCATE questions; TRUNCATE quizzes; TRUNCATE users; SET FOREIGN_KEY_CHECKS=1;"`, { stdio: ['ignore', 'pipe', 'pipe'] })
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
const SINGLE = [ // [题干, 正确答案下标, 选项, 解析]
  ['某公司安全巡检发现：数据库服务器的 MySQL 3306 端口对公网开放，且管理员账号使用弱口令。若攻击者利用该缺陷发起攻击，最直接的后果是？', 1, ['Web 服务器 CPU 被占满导致宕机', '数据库被暴力破解，业务数据被窃取或篡改', '用户浏览器缓存被清空', '域名解析记录被批量修改'], '数据库端口暴露+弱口令是最典型的入侵入口，攻击者可远程爆破登录后拖库。'],
  ['开发人员在登录页面的用户名输入框中提交「admin\' OR \'1\'=\'1」--」，竟然绕过密码校验直接进入了系统后台。该攻击成功的原因是？', 0, ['用户输入被拼接进 SQL 语句，改变了查询的逻辑', '服务器管理员密码被暴力破解', '浏览器同源策略被禁用', 'TLS 证书校验失败'], '恶意构造的 SQL 片段使 WHERE 条件恒真，是典型的 SQL 注入攻击。'],
  ['某论坛评论区被植入一段 <script> 代码，任何浏览该帖子的用户浏览器都会自动执行这段脚本，会话 Cookie 被悄悄发送到攻击者的服务器。该攻击属于？', 2, ['SQL 注入', 'DDoS 攻击', '存储型 XSS 跨站脚本攻击', '中间人攻击'], '恶意脚本被服务端存储后再呈现给访问者，属于存储型 XSS；防御要点是输出编码。'],
  ['研发团队需要对一批共计 200GB 的视频素材做高速加解密，且收发双方已通过线下渠道安全交换了密钥。下列最合适的算法是？', 3, ['RSA 非对称加密', 'ECC 椭圆曲线加密', 'DSA 数字签名算法', 'AES 对称加密'], '大流量数据加密首选对称算法（性能高），AES 是当前标准；RSA/ECC 适合密钥交换与签名。'],
  ['企业网络边界部署了一台设备，依据管理员预先配置的规则集（源/目的 IP、端口、协议）对进出流量逐包检查并决定放行或丢弃。该设备是？', 1, ['IDS 入侵检测系统', '防火墙 Firewall', '杀毒软件', 'VPN 网关'], '防火墙的核心职能是访问控制；IDS 以旁路检测告警为主，不直接阻断流量。'],
  ['攻击者拿到 A 网站泄露的 500 万条「账号-密码」记录，用这些凭据批量尝试登录 B 银行网站，大量用户因「多站同密码」而账户被盗。这种攻击称为？', 2, ['SYN 洪水攻击', 'DNS 重绑定攻击', '撞库攻击', '提权攻击'], '撞库的本质是密码复用：一处泄露、处处失守。不同网站使用不同密码是最有效的防范。'],
  ['关于 SSL/TLS 协议在网络协议栈中的位置与作用，下列说法正确的是？', 1, ['工作在网络层，负责 IP 包路由', '位于传输层之上、应用层之下，为上层协议提供加密与完整性保护', '工作在物理层，负责信号编码', '是应用层之上的展示层渲染协议'], 'TLS 为 HTTP/SMTP 等应用协议提供端到端加密，HTTPS = HTTP over TLS。'],
  ['某历史系统仍使用 MD5 对用户口令做单向哈希后存储。从现代密码学角度，MD5 不适合用于口令保护的主要原因是？', 0, ['已可高效构造碰撞，且算力发展使其极易被彩虹表与 GPU 暴力破解', '运算速度太慢影响登录', '生成的摘要太长浪费存储', '不支持中文字符'], 'MD5 的抗碰撞性已被攻破，且 128 位摘要面对 GPU 离线爆破强度不足；应换 bcrypt/Argon2 加盐。'],
  ['用户收到短信：「尊敬的客户，您的银行账户存在异常，请点击 http://bank-verify.cc 完成身份核验」。链接指向的页面与官网视觉完全一致。该攻击属于？', 3, ['蓝牙近场攻击', 'U 盘摆渡攻击', '系统更新劫持', '钓鱼攻击（Phishing）'], '仿冒官网+诱导输入凭据是典型钓鱼；核实域名、不点短信链接是基本防线。'],
  ['某电商平台在 Web 服务器前置部署了一套安全设备，通过解析 HTTP 请求的参数、头部与载荷特征，识别并阻断 SQL 注入、XSS、扫描器探测等攻击。该设备是？', 2, ['广域网防火墙', '包过滤路由器', 'Web 应用防火墙（WAF）', '负载均衡器'], 'WAF 工作在应用层，理解 HTTP 语义，与传统网络层防火墙互补。'],
  ['为防止攻击者对登录接口进行口令枚举（Credential Stuffing），下列组合措施中最有效的是？', 1, ['要求用户名长度超过 20 位', '登录失败次数限制+图形验证码+异地登录告警', '建议用户更换浏览器', '每周定期重启服务器'], '限制尝试频率提高爆破成本，验证码阻断自动化脚本，告警及时发现异常登录。'],
  ['新入职的财务专员岗位仅需要查询报销数据，管理员图省事直接给其开通了财务系统全部模块的增删改权限。该做法违反了？', 2, ['深度防御原则', '职责分离原则', '最小权限原则（Least Privilege）', '默认开放原则'], '最小权限要求仅授予完成任务所必需的权限，超配权限会在账号被盗时放大损失。'],
  ['攻击者冒充 IT 运维人员致电公司前台，声称「远程排查网络故障」，以着急加班为由施压，成功套取了前台的 VPN 账号与密码。该攻击手段属于？', 0, ['社会工程学攻击', '中间人攻击', '重放攻击', '水坑攻击'], '社会工程学利用「人的信任、恐惧、服从权威」等心理弱点，技术防护无法完全覆盖，需安全意识培训。'],
  ['某办公软件爆出正在被野外利用的高危漏洞，攻击者已用于投放木马，而厂商尚未发布安全补丁。此类漏洞被称为？', 3, ['已归档漏洞', '零影响漏洞', '低危已修复漏洞', '零日漏洞（Zero-day）'], '「零日」指留给防御者的准备时间为零；无补丁阶段的缓解手段包括限制访问、虚拟补丁等。'],
  ['公司内网多台主机频繁掉线、网关时通时断，抓包发现大量伪造的免费 ARP 应答报文将网关 MAC 指向了内网另一台主机。该攻击发生在？', 1, ['应用层 HTTP', '局域网链路层（ARP 欺骗）', '传输层 TCP', '物理层线缆'], 'ARP 欺骗利用协议无认证的缺陷在同一广播域内冒充网关，可实施流量劫持与嗅探。'],
  ['用户在浏览器输入正确的网上银行网址，却被引导到一个界面完全相同的仿冒站点并输入了密码。经排查，DNS 查询返回了错误的 IP 地址。该攻击称为？', 2, ['XSS 脚本注入', '本地hosts劫持', 'DNS 劫持', 'CDN 缓存污染'], 'DNS 劫持让域名解析指向攻击者服务器；使用可信 DNS/DoH 可缓解。'],
  ['用户访问某网站时，浏览器出现「您的连接不是私密连接」警告页面。检查发现该站点的 SSL 证书已过期。浏览器此时的正确行为是？', 0, ['向用户提示风险并阻止/建议不要继续访问', '自动为该网站续期证书', '静默放行不提示', '切换到 HTTP 明文加速访问'], '证书过期使身份无法验证，浏览器必须显式告警，用户不应忽略警告继续访问。'],
  ['安全团队在内网部署了一台刻意配置弱口令、看似存有「客户资料」的服务器，实际用于吸引攻击者并记录其手法与来源。该设施称为？', 1, ['数据备份节点', '蜜罐（Honeypot）', '跳板机', '日志归档服务器'], '蜜罐以「假目标」换取攻击情报，为真实资产争取响应时间。'],
  ['总部与异地分支机构需要通过公共互联网安全互访内网业务系统，要求传输数据全程加密且对用户透明。最合适的技术方案是？', 2, ['将全部端口映射到公网', '关闭防火墙直连', '部署 VPN 建立加密隧道', '改用 HTTP 明文访问'], 'VPN 在公网上构建加密隧道，兼顾机密性与接入便捷；直接暴露内网服务是大忌。'],
  ['为防止用户口令哈希被彩虹表批量破解，系统在哈希前为每个口令附加一段随机「盐（Salt）」并与哈希值一同存储。盐的作用是？', 3, ['增加口令的明文长度', '提升哈希运算速度', '压缩哈希结果节省空间', '使相同口令产生不同哈希，使预计算的彩虹表失效'], '加盐后攻击者必须为每个盐值单独重建彩虹表，破解成本呈用户数量级放大。'],
]
const MULTI = [ // [题干, 正确答案下标数组, 选项, 解析]
  ['参照 OWASP Top 10，下列哪些属于常见的 Web 应用安全风险？', [0, 1, 3], ['SQL 注入（Injection）', '失效的访问控制与 XSS', 'TCP 三次握手设计缺陷', '跨站请求伪造（CSRF）'], 'OWASP Top 10 中的注入类、XSS、CSRF、访问控制失效均属高危；TCP 握手是正常协议机制。'],
  ['安全团队制定口令策略时，下列哪些属于强密码的必要特征？', [0, 2, 3], ['长度不少于 12 位', '包含用户生日便于记忆', '大小写字母+数字+符号混合', '不包含字典常见单词与键盘序列'], '生日、姓名拼音、qwerty/123456 等模式都在攻击者字典前列，长度与随机性才是核心。'],
  ['下列算法中，属于「对称加密」的有哪些？', [0, 2], ['DES（数据加密标准）', 'RSA', 'SM4（国密对称算法）', 'ECC 椭圆曲线'], 'DES/SM4/AES 加解密使用同一密钥属对称算法；RSA/ECC 基于数学难题，属非对称算法。'],
  ['网站从 HTTP 升级到 HTTPS 后，获得了哪些安全能力？', [0, 1, 2], ['传输内容加密，防止被窃听', '通过证书验证服务器身份，防冒充', '防止内容在传输中被篡改（完整性校验）', '自动检测并查杀访问者电脑中的病毒'], 'HTTPS 解决传输安全三问题：窃听、冒充、篡改；终端杀毒不在其职责范围。'],
  ['下列日常上网行为中，哪些存在明显安全隐患？', [0, 2, 3], ['连接公共 WiFi 后直接登录网上银行', '仅从手机官方应用商店下载 App', '多个网站使用同一个密码', '点击短信中的不明短链接并填写个人信息'], '官方商店有上架审核相对可控；其余三项分别是嗅探、撞库、钓鱼的高危场景。'],
  ['渗透测试人员进行信息收集与流量分析时，下列哪些是业内常用工具？', [0, 2, 3], ['Nmap（端口与服务扫描）', 'Photoshop', 'Wireshark（抓包分析）', 'Burp Suite（Web 流量拦截）'], 'Nmap/Wireshark/Burp 是渗透测试三件套；Photoshop 是图像处理软件。'],
  ['关于 DDoS 分布式拒绝服务攻击，下列描述正确的有哪些？', [0, 1, 3], ['通常由大量被控制的傀儡机（Botnet）发起', '目标是耗尽受害者的带宽、连接或计算资源', '主要目的是窃取数据库中的客户资料', '攻击流量可达数百 Gbps，造成服务不可用'], 'DDoS 以「打瘫服务」为目的而非窃密；僵尸网络+超大流量是其典型特征。'],
  ['保护个人账号信息安全，下列哪些做法是推荐的？', [0, 1, 3], ['为重要账号开启双因素认证（2FA）', '定期更换密码且不复用旧密码', '在朋友圈晒身份证和银行卡照片', '不在陌生网站填写手机号与验证码'], '2FA 使「仅凭密码」失守也不至于沦陷；晒证件与随意提交验证码都是信息泄露高发点。'],
  ['下列协议中，具备加密或安全认证能力的有哪些？', [0, 1, 3], ['TLS 1.3', 'SSH', 'HTTP 明文协议', 'Kerberos'], 'TLS 保护传输、SSH 保护远程登录、Kerberos 做域内认证；HTTP 明文无任何保护。'],
  ['下列哪些属于恶意软件（Malware）的典型类型？', [0, 1, 3], ['木马（Trojan）伪装成正常程序窃取信息', '蠕虫（Worm）可自我复制主动传播', '杀毒软件 Antivirus', '勒索软件（Ransomware）加密文件勒索赎金'], '杀毒软件是防御工具；木马重「伪装潜伏」、蠕虫重「主动传播」、勒索重「破坏勒索」。'],
  ['企业建设日志审计体系，其安全价值包括哪些？', [0, 1, 2], ['攻击发生后追溯入侵路径与影响范围', '通过异常日志及时发现正在进行的攻击', '满足等级保护等合规要求', '直接提升 WiFi 信号强度'], '日志是取证与检测的基石，也是等保合规硬指标；与无线信号无关。'],
  ['针对 XSS 跨站脚本攻击，下列哪些是有效的防御手段？', [0, 2, 3], ['对输出到页面的内容做编码转义（Output Encoding）', '关闭 Web 应用防火墙', '配置 CSP 内容安全策略限制脚本来源', '为 Cookie 设置 HttpOnly 属性'], '输出编码断绝注入、CSP 限制脚本执行、HttpOnly 阻止脚本读 Cookie；关闭 WAF 是反向操作。'],
  ['信息安全中的 CIA 三要素包括哪些？', [0, 1, 3], ['机密性（Confidentiality）', '完整性（Integrity）', '高并发（Availability 指高性能）', '可用性（Availability）'], 'CIA=机密性+完整性+可用性，三者共同构成信息安全的基本评价维度。'],
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
  show_answer: true, show_analysis: true, show_ranking: true,
}, at)).data
if (!quiz?.id) throw new Error('建赛失败')

let sort = 0
const mk = async (type, pool, idx, score, required, timeLimit) => {
  const [content, correct, options, analysis] = pool[idx]
  const correctArr = Array.isArray(correct) ? correct : [correct]
  sort++
  const r = await j('POST', `/api/admin/quiz/${quiz.code}/questions`, {
    type, content,
    answer: correctArr.map(i => L[i]).sort().join(''),
    analysis: analysis || '答对得本题分值；抢答题答错扣对应分值。',
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

console.log(`✅ 已生成「计分规则验证赛」#${quiz.code}（30 题，共 100 分）`)
console.log('   必答题 15 题 40 分：10 单选×2 + 5 多选×4（答错 0 分）')
console.log('   抢答题 15 题 60 分：10 单选×3 + 5 多选×6（答错扣对应分值：单选-3、多选-6）')
console.log('   选手（用户名=姓名拼音，密码=用户名+12345）：')
console.log('     zhangwei/张伟  liuyang/刘洋  chenjing/陈静  wangfang/王芳  zhaolei/赵磊')
console.log(`   流程：控制台 ▶开始 → 必答题直接作答；到抢答题时点 ⚡开始抢答 → 抢到者作答（答错倒扣）`)
