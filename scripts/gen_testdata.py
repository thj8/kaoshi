#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
生成测试数据脚本：网络安全知识竞赛

- 单选 20 题 / 多选 20 题 / 判断 20 题，共 60 题
- 每个类型：前 10 题必答（required=true），后 10 题抢答（题干带【抢答】标记，required=false）
- 通过管理端 API 创建答题并写入全部题目

用法：
    python3 gen_testdata.py [BASE_URL]

    BASE_URL 默认 http://localhost:13000（nginx 入口，外部测试传 http://<服务器IP>:13000）

环境变量：
    KAOSHI_ADMIN_USER / KAOSHI_ADMIN_PASS   管理员账号（默认 admin/admin123）
"""

import json
import os
import sys
import urllib.request

BASE = (sys.argv[1] if len(sys.argv) > 1 else os.environ.get("KAOSHI_BASE", "http://localhost:13000")).rstrip("/")
ADMIN_USER = os.environ.get("KAOSHI_ADMIN_USER", "admin")
ADMIN_PASS = os.environ.get("KAOSHI_ADMIN_PASS", "admin123")


def req(method, path, body=None, token=None):
    r = urllib.request.Request(
        BASE + path, method=method,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Content-Type": "application/json"},
    )
    if token:
        r.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(r, timeout=15) as resp:
        data = json.loads(resp.read())
    if data.get("code") != 0:
        raise RuntimeError(f"{method} {path} 失败: {data.get('msg')}")
    return data["data"]


# ==================== 题库 ====================

SINGLE = [
    # ---- 前 10：必答 ----
    ("HTTP 默认使用的端口是？", ["21", "80", "443", "3306"], "B", "HTTP 默认使用 TCP 80 端口，443 是 HTTPS，21 是 FTP，3306 是 MySQL。"),
    ("HTTPS 默认使用的端口是？", ["80", "8080", "443", "53"], "C", "HTTPS = HTTP over TLS，默认 TCP 443 端口。"),
    ("SSH 服务默认使用的端口是？", ["21", "22", "23", "25"], "B", "SSH 默认 22 端口；21 是 FTP，23 是 Telnet（明文，不安全），25 是 SMTP。"),
    ("DNS 服务默认使用的端口是？", ["53", "54", "80", "110"], "A", "DNS 默认使用 TCP/UDP 53 端口。"),
    ("TCP 三次握手中，第二步服务端返回的报文是？", ["SYN", "ACK", "SYN+ACK", "FIN"], "C", "三次握手：客户端 SYN → 服务端 SYN+ACK → 客户端 ACK。"),
    ("SQL 注入攻击的主要目标是？", ["操作系统内核", "数据库", "浏览器缓存", "路由器固件"], "B", "SQL 注入通过拼接恶意 SQL 语句欺骗数据库执行，目标是后端数据库。"),
    ("通过在网页中注入恶意脚本、在用户浏览器中执行的攻击是？", ["XSS", "CSRF", "SSRF", "DDoS"], "A", "XSS（跨站脚本）把恶意脚本注入页面，在受害者浏览器执行。"),
    ("存储用户密码最安全的做法是？", ["MD5 哈希", "明文存储", "加盐慢哈希（如 bcrypt）", "Base64 编码"], "C", "MD5 速度快易被彩虹表/碰撞破解；应使用加盐的慢哈希算法如 bcrypt/scrypt/Argon2。"),
    ("ARP 欺骗主要发生在 OSI 模型的哪一层？", ["物理层", "数据链路层", "传输层", "应用层"], "B", "ARP 工作在数据链路层（第二层），用于 IP 到 MAC 地址的解析。"),
    ("最常用的端口扫描工具是？", ["Wireshark", "Nmap", "Burp Suite", "Sqlmap"], "B", "Nmap 是经典端口扫描与服务识别工具；Wireshark 抓包、Burp 是 Web 代理、Sqlmap 注入。"),
    # ---- 后 10：抢答 ----
    ("SYN Flood 属于哪一类攻击？", ["拒绝服务攻击", "权限提升", "钓鱼攻击", "社会工程学"], "A", "SYN Flood 利用 TCP 握手缺陷发送大量半连接请求耗尽服务器资源，属于 DoS/DDoS。"),
    ("TLS/SSL 协议工作在？", ["传输层之上、应用层之下", "物理层", "网络层", "数据链路层"], "A", "TLS 位于传输层之上为应用层协议（HTTP/SMTP 等）提供加密与完整性保护。"),
    ("Linux 下查看当前开放监听端口的命令是？", ["netstat -antp / ss -lntp", "ls -la", "cat /etc/passwd", "top"], "A", "netstat -antp 或 ss -lntp 可列出监听端口及对应进程。"),
    ("ICMP 协议的主要作用是？", ["文件传输", "网络控制与差错报告", "邮件收发", "域名解析"], "B", "ICMP 用于传递控制消息（如 ping、TTL 超时、目标不可达）。"),
    ("使用密码字典逐一尝试口令的攻击方式称为？", ["暴力破解", "SQL 注入", "XSS", "蠕虫传播"], "A", "暴力破解/字典攻击通过大量尝试口令获得访问权限。"),
    ("HTTPS 数字证书主要用于验证？", ["服务器身份，防中间人攻击", "网页排版", "用户网速", "浏览器版本"], "A", "CA 签发的证书证明服务器身份，防止中间人伪造站点。"),
    ("DNS 劫持最直接的后果是？", ["域名解析到攻击者控制的 IP", "硬盘被格式化", "CPU 占用降低", "内存条损坏"], "A", "DNS 劫持把域名解析引向恶意 IP，常用于钓鱼与流量劫持。"),
    ("WAF 的主要作用是？", ["过滤恶意 HTTP 请求保护 Web 应用", "查杀文件病毒", "磁盘加密", "数据库备份"], "A", "WAF（Web 应用防火墙）部署在 Web 前端，拦截 SQL 注入/XSS 等恶意请求。"),
    ("3389 端口对应的服务是？", ["Windows 远程桌面", "FTP", "SMTP", "MySQL"], "A", "RDP 远程桌面默认 3389 端口，公网暴露极易被爆破，建议限源访问。"),
    ("0day 漏洞是指？", ["厂商已知且已发布补丁的漏洞", "被发现但厂商尚无补丁的漏洞", "已彻底修复的漏洞", "虚构的漏洞"], "B", "0day 指被披露时厂商还没有可用补丁的漏洞，风险极高。"),
]

MULTIPLE = [
    # ---- 前 10：必答 ----
    ("以下哪些属于 Web 安全漏洞？", ["SQL Injection", "XSS", "SSRF", "FTP"], "ABC", "SQLi/XSS/SSRF 都是典型 Web 漏洞，FTP 是协议本身。"),
    ("以下哪些属于对称加密算法？", ["AES", "DES", "RSA", "3DES"], "ABD", "AES/DES/3DES 是对称加密；RSA 是非对称加密。"),
    ("以下哪些属于非对称加密算法？", ["RSA", "ECC", "AES", "DH"], "ABD", "RSA/ECC/DH 为非对称算法；AES 为对称算法。"),
    ("以下哪些属于常见 Web 攻击方式？", ["CSRF", "XSS", "文件上传漏洞", "缓冲区溢出"], "ABC", "CSRF/XSS/文件上传针对 Web 应用；缓冲区溢出主要针对底层程序。"),
    ("XSS 的常见类型包括？", ["存储型", "反射型", "DOM 型", "盲注型"], "ABC", "XSS 分存储型、反射型、DOM 型三类。"),
    ("SQL 注入的常见利用方式包括？", ["联合查询注入", "布尔盲注", "时间盲注", "堆叠查询注入"], "ABCD", "四种都是常见 SQL 注入利用手法。"),
    ("以下哪些属于哈希（散列）算法？", ["MD5", "SHA256", "SM3", "Base64"], "ABC", "MD5/SHA256/SM3 是哈希算法；Base64 只是编码方式，不是哈希。"),
    ("防御 CSRF 攻击的有效措施包括？", ["使用 Anti-CSRF Token", "校验 Referer", "Cookie 设置 SameSite", "过滤 SQL 关键字"], "ABC", "Token/Referer 校验/SameSite 针对 CSRF；过滤 SQL 关键字防的是注入。"),
    ("SSRF 漏洞可能被利用的协议包括？", ["http/https", "file", "dict", "gopher"], "ABCD", "SSRF 可借助多种协议探测内网、读取文件甚至攻击内网服务。"),
    ("以下哪些是端口扫描工具？", ["Nmap", "Masscan", "Burp Suite", "Zmap"], "ABD", "Nmap/Masscan/Zmap 是扫描器；Burp Suite 是 Web 安全测试代理。"),
    # ---- 后 10：抢答 ----
    ("Linux 服务器安全加固措施包括？", ["关闭无用服务", "修改 SSH 默认端口", "禁止 root 直接远程登录", "及时安装安全补丁"], "ABCD", "四项都是基础加固最佳实践。"),
    ("钓鱼邮件的常见特征包括？", ["仿造官方域名", "制造紧迫感（如账号将冻结）", "诱导点击陌生链接", "索取账号密码"], "ABCD", "仿冒域名、紧迫话术、恶意链接、索取凭据是钓鱼四大典型特征。"),
    ("XSS 的有效防御措施包括？", ["输出编码/转义", "配置 CSP", "Cookie 设置 HttpOnly", "数据库参数化查询"], "ABC", "前三个针对 XSS；参数化查询防 SQL 注入。"),
    ("SQL 注入的有效防御措施包括？", ["参数化查询/预编译", "部署 WAF", "数据库账号最小权限", "过滤校验用户输入"], "ABCD", "多层防御：编码层参数化、WAF 拦截、权限收敛、输入校验。"),
    ("常见的中间人攻击手段包括？", ["ARP 欺骗", "DNS 欺骗", "SSL 剥离", "XSS"], "ABC", "ARP/DNS 欺骗与 SSL 剥离是典型 MITM 手段；XSS 属于注入类攻击。"),
    ("Linux 系统中常见的安全日志位置包括？", ["/var/log/auth.log", "/var/log/syslog", "/var/log/secure", "/tmp"], "ABC", "auth.log/secure 记录认证事件，syslog 记录系统日志；/tmp 是临时目录。"),
    ("良好的密码策略包括？", ["长度不少于 8 位", "大小写字母+数字+符号混合", "定期更换", "多个系统使用同一密码"], "ABC", "多系统共用密码一旦泄露会引发撞库连锁风险。"),
    ("防御文件上传漏洞的措施包括？", ["后缀白名单校验", "文件重命名", "上传目录禁止执行脚本", "校验文件类型"], "ABCD", "四项组合构成文件上传安全基线。"),
    ("DDoS 攻击的常见形式包括？", ["SYN Flood", "UDP Flood", "ICMP Flood", "HTTP Flood"], "ABCD", "协议层（SYN/UDP/ICMP）与应用层（HTTP）洪水都是 DDoS 常见形式。"),
    ("应急响应的标准步骤包括？", ["隔离受影响主机", "保留现场取证", "清除恶意程序", "复盘并加固"], "ABCD", "准备-检测-遏制-根除-恢复-复盘是标准 PDCERF 流程要点。"),
]

JUDGE = [
    # ---- 前 10：必答 ----
    ("HTTPS 可以防止传输内容被窃听。", "A", "HTTPS 基于 TLS 加密传输，可有效防窃听与篡改。"),
    ("MD5 是加密算法且绝对不可破解。", "B", "MD5 是哈希算法而非加密，且已被碰撞攻击破解，不能用于安全场景。"),
    ("将 MySQL 的 3306 端口直接开放到公网是安全实践。", "B", "数据库端口暴露公网极易被扫描爆破，应限内网或加白名单。"),
    ("部署了 WAF 就不需要再修复代码层漏洞。", "B", "WAF 只是外层缓解措施，纵深防御仍需修复代码。"),
    ("强口令应包含大小写字母、数字和特殊符号。", "A", "多字符集组合并保证长度可显著提高抗爆破能力。"),
    ("参数化查询可以有效防御 SQL 注入。", "A", "预编译将数据与语句分离，是最有效的注入防御手段。"),
    ("无线网络使用 WEP 加密是安全的。", "B", "WEP 已被彻底破解，应至少使用 WPA2/WPA3。"),
    ("端口扫描通常是攻击前的侦察行为。", "A", "攻击者通过扫描探测开放端口与服务版本，为后续利用做准备。"),
    ("杀毒软件能够查杀所有未知病毒。", "B", "未知威胁（0day）无法靠特征库识别，需结合 EDR/行为检测。"),
    ("及时安装安全补丁是防御已知漏洞的关键手段。", "A", "绝大多数入侵利用的是已公开且有补丁的漏洞。"),
    # ---- 后 10：抢答 ----
    ("CSRF 攻击利用了浏览器自动携带 Cookie 的机制。", "A", "CSRF 借助浏览器对站点的自动认证（Cookie）伪造用户请求。"),
    ("SSH 比 Telnet 安全，因为 SSH 对传输内容加密。", "A", "Telnet 明文传输，SSH 加密并校验完整性。"),
    ("内网系统不需要做任何安全防护。", "B", "内网同样面临横向渗透威胁，需最小权限与访问控制。"),
    ("使用 HTTPS 的网站一定不存在 XSS 漏洞。", "B", "HTTPS 只保证传输安全，无法防御应用层注入漏洞。"),
    ("Base64 是一种加密算法。", "B", "Base64 是可逆编码，无密钥、无安全性，不属于加密。"),
    ("开启双因素认证可以提升账户安全性。", "A", "2FA 在口令外增加第二因子，即使口令泄露也多一道防线。"),
    ("日志审计对入侵检测没有帮助。", "B", "日志是检测入侵、溯源取证的核心数据来源。"),
    ("SYN Flood 利用 TCP 三次握手缺陷消耗服务器资源。", "A", "大量伪造半连接占据 backlog 队列，导致拒绝服务。"),
    ("密码设置得越简单越好记就越安全。", "B", "简单口令极易被字典/爆破破解，长度与复杂度才是关键。"),
    ("社会工程学攻击主要利用人的心理弱点。", "A", "社工攻击绕过技术防线，直接利用信任、紧迫感等心理弱点。"),
]


def build_questions():
    """组装为 API 题目结构：每个类型前 10 必答，后 10 抢答"""
    questions = []

    def opts(labels_contents):
        return [{"label": chr(65 + i), "content": c} for i, c in enumerate(labels_contents)]

    for i, (content, choices, ans, analysis) in enumerate(SINGLE):
        rush = i >= 10
        questions.append({
            "type": "single",
            "content": ("【抢答】" if rush else "") + content,
            "options": opts(choices),
            "answer": ans,
            "analysis": analysis,
            "score": 10 if not rush else 15,
            "required": not rush,
            "time_limit": 0,
        })

    for i, (content, choices, ans, analysis) in enumerate(MULTIPLE):
        rush = i >= 10
        questions.append({
            "type": "multiple",
            "content": ("【抢答】" if rush else "") + content,
            "options": opts(choices),
            "answer": ans,
            "analysis": analysis,
            "score": 20 if not rush else 30,
            "required": not rush,
            "time_limit": 0,
        })

    for i, (content, ans, analysis) in enumerate(JUDGE):
        rush = i >= 10
        questions.append({
            "type": "judge",
            "content": ("【抢答】" if rush else "") + content,
            "options": opts(["正确", "错误"]),
            "answer": ans,
            "analysis": analysis,
            "score": 5 if not rush else 8,
            "required": not rush,
            "time_limit": 0,
        })
    return questions


def main():
    print(f"目标服务: {BASE}")
    token = req("POST", "/api/admin/login", {"username": ADMIN_USER, "password": ADMIN_PASS})["token"]

    quiz = req("POST", "/api/admin/quiz", {
        "title": "网络安全知识竞赛（测试数据）",
        "description": "共 60 题：单选/多选/判断各 20 题。每种题型前 10 题必答，后 10 题为抢答题（题干带【抢答】标记）。",
        "mode": "rush",
        "per_question_time": 30,
        "total_time": 0,
        "show_answer": True,
        "show_analysis": True,
        "show_ranking": True,
        "rush_enabled": True,
        "rush_winner_count": 1,
        "rush_time": 10,
        "rush_answer_time": 20,
        "rush_bonus_score": 5,
        "rush_wrong_score": 5,
    }, token)
    quiz_id, invite = quiz["id"], quiz["invite_code"]
    print(f"已创建答题: id={quiz_id} 邀请码={invite}")

    questions = build_questions()
    for i, q in enumerate(questions, 1):
        req("POST", f"/api/admin/quiz/{quiz_id}/questions", q, token)
        if i % 10 == 0 or i == len(questions):
            print(f"  题目进度: {i}/{len(questions)}")

    total = len(questions)
    req_cnt = sum(1 for q in questions if q["required"])
    rush_cnt = total - req_cnt
    print("\n========== 完成 ==========")
    print(f"答题 ID  : {quiz_id}")
    print(f"邀请码   : {invite}")
    print(f"题目总数 : {total}（必答 {req_cnt} + 抢答 {rush_cnt}）")
    print(f"管理控制台: {BASE}/admin/quiz/{quiz_id}/console")
    print(f"用户加入 : {BASE}/join （昵称 + 邀请码 {invite}）")


if __name__ == "__main__":
    main()
