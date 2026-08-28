# Git 历史清洗实操记录（git filter-repo）

> 场景：仓库准备推送到公开 GitHub 前，需要从**全部提交历史**中移除敏感文件与硬编码密钥。
> 背景：`客户演示指南.md`（含真实管理员密码/内网 IP）与 `bug.md`（渗透测试报告）曾误入库后删除；更早期提交的 `docker-compose.yml` 与 E2E 脚本中硬编码过真实密码（后才改为 `.env` 引用）。
> 结论：删除工作区文件 ≠ 从历史移除，**必须重写历史**。

## 0. 前置检查

```bash
# 确认 filter-repo 是否可用（未安装则：apt-get install -y git-filter-repo）
git filter-repo --version

# 确认仓库从未推送过（若已推送过公开远程，清洗后需 force push 且密钥应视为已泄露、必须轮换）
git remote -v

# 定位敏感文件出现在哪些提交
git log --all --oneline -- "docs/客户演示指南.md" bug.md
```

## 1. 备份（必做）

历史重写不可逆，先打全量 bundle 备份：

```bash
git bundle create /root/kaoshi-pre-filter.bundle --all
```

> 备份含清洗前的敏感内容，确认推送成功后应删除，不要随意外发。

## 2. 从全部历史中删除整个文件

```bash
git filter-repo --force --invert-paths \
  --path "docs/客户演示指南.md" \
  --path "bug.md" \
  --path "b4035dcc-29d0-4dc3-89f9-6f60af21e5e2.png" \
  --path "download.png"
```

要点：

- `--invert-paths` = 「删除这些路径」；不加才是「只保留这些路径」
- 在原仓库（非 fresh clone）执行必须加 `--force`
- 已从工作区删除、但历史里存在的大文件（如临时截图）可顺带清理，同时给仓库瘦身
- filter-repo 会移除 remote 配置（防误推旧历史），推送前需重新 `git remote add`

## 3. 替换历史中的硬编码密钥（文件本身要保留时）

核心文件（如 docker-compose.yml）不能整个删除，只能改写其中的密钥字符串。
先扫描确认泄露范围（**哪些提交的哪些文件**命中密钥）：

```bash
for c in $(git rev-list --all); do
  git grep -l '<密钥片段>' $c 2>/dev/null
done | sort -u | cut -d: -f2 | sort -u
```

编写替换规则文件（每行一个待替换字符串，默认替换为 `***REMOVED***`）：

```bash
cat > /tmp/replace-secrets.txt <<'EOF'
<管理员密码>
<MySQL root 密码>
<Redis 密码>
<JWT secret>
<内网IP，如 10.x.x.x>
EOF

git filter-repo --force --replace-text /tmp/replace-secrets.txt
rm /tmp/replace-secrets.txt   # 规则文件含密钥本体，用完即删
```

## 4. 验证（推公开前的最后一道关）

```bash
# ① 敏感文件的历史应为空
git log --all --oneline -- "docs/客户演示指南.md" bug.md

# ② 全历史逐提交扫描密钥片段，应无任何输出
for s in <密钥片段1> <密钥片段2> <内网IP>; do
  git rev-list --all | while read c; do
    git grep -q "$s" $c 2>/dev/null && echo "LEAK: $s @ $c"
  done
done

# ③ 完整性检查：提交数、分支、工作区不受影响，HEAD 现行文件内容未被误伤
git log --oneline | wc -l
git branch
git status --short
```

## 5. 推送到公开远程

```bash
# 首次 SSH 连接需先信任 GitHub host key
ssh-keyscan -t ed25519,rsa github.com >> ~/.ssh/known_hosts
ssh -T git@github.com        # 验证认证身份

git remote add origin git@github.com:<账号>/<仓库>.git
git push -u origin --all
```

## 注意事项

| 事项 | 说明 |
|---|---|
| **commit hash 全变** | 历史重写后所有（受影响链上的）hash 改变，此前引用的旧 hash 全部作废 |
| **未推送过 → 无需轮换密钥** | 本仓库清洗前从未推送，密钥未外泄，`.env` 凭据可继续使用；**若曾推送过公开远程，清洗后也必须视为已泄露，全部轮换** |
| **协作仓库需协调** | 其他人基于旧历史的克隆无法直接 pull，需重新 clone |
| **仅清文件不够** | 还要想到历史中的硬编码密钥（本例即漏网之鱼，靠第 3 步扫描兜底） |
