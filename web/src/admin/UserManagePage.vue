<template>
  <div class="page">
    <h1 style="margin-bottom: 14px">👥 用户管理</h1>
    <div style="display: flex; gap: 10px; margin-bottom: 16px">
      <input
        v-model="keyword"
        class="input"
        style="max-width: 280px"
        placeholder="搜索用户名/昵称..."
        @input="debouncedLoad"
      />
      <button class="btn btn-primary" @click="openCreate">＋ 新增用户</button>
    </div>

    <div class="card" style="overflow: auto">
      <table class="utable">
        <thead>
          <tr>
            <th>ID</th>
            <th>用户名</th>
            <th>昵称</th>
            <th>正确率</th>
            <th style="width: 150px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="5" class="text-dim" style="text-align: center; padding: 30px">加载中...</td>
          </tr>
          <tr v-else-if="users.length === 0">
            <td colspan="5" class="text-dim" style="text-align: center; padding: 30px">暂无用户</td>
          </tr>
          <tr v-for="u in users" :key="u.id">
            <td class="text-dim">{{ u.id }}</td>
            <td><code style="background: var(--card-2); padding: 2px 8px; border-radius: 6px">{{ u.username || '—' }}</code></td>
            <td><b>{{ u.nickname }}</b></td>
            <td>{{ correctRate(u) }}%</td>
            <td>
              <div style="display: flex; gap: 6px">
                <button class="btn btn-ghost" style="padding: 5px 10px; font-size: 13px" @click="openDetail(u)">明细</button>
                <button class="btn btn-ghost" style="padding: 5px 10px; font-size: 13px" @click="openEdit(u)">编辑</button>
                <button class="btn btn-danger" style="padding: 5px 10px; font-size: 13px" @click="delUser(u)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 新增/编辑弹窗 -->
    <div v-if="editing" class="modal" @click.self="editing = false">
      <div class="card modal-body">
        <h3 style="margin-bottom: 14px">{{ editTarget ? `编辑用户 #${editTarget?.id}` : '新增用户' }}</h3>
        <div v-if="!editTarget" style="margin-bottom: 12px">
          <label class="flbl">用户名 *</label>
          <input v-model="editForm.username" class="input" placeholder="登录用户名" />
        </div>
        <div style="margin-bottom: 12px">
          <label class="flbl">昵称 {{ editTarget ? '' : '*' }}</label>
          <input v-model="editForm.nickname" class="input" placeholder="排行榜展示昵称" />
        </div>
        <div style="margin-bottom: 4px">
          <label class="flbl">{{ editTarget ? '新密码（留空则不修改）' : '密码 *' }}</label>
          <input v-model="editForm.password" class="input" type="password" placeholder="至少 4 位" />
        </div>
        <p v-if="editErr" style="color: var(--danger); font-size: 13px; margin-top: 8px">{{ editErr }}</p>
        <div style="display: flex; gap: 10px; margin-top: 16px">
          <button class="btn btn-ghost" style="flex: 1" @click="editing = false">取消</button>
          <button class="btn btn-primary" style="flex: 1" @click="saveUser">保存</button>
        </div>
      </div>
    </div>

    <!-- 明细弹窗 -->
    <div v-if="detail" class="modal" @click.self="detail = null">
      <div class="card modal-body" style="max-height: 80vh; overflow: auto">
        <h3 style="margin-bottom: 4px">{{ detail.user.nickname }} <span class="text-dim" style="font-size: 13px">#{{ detail.user.id }}</span></h3>
        <p class="text-dim" style="font-size: 13px; margin-bottom: 16px">参加记录</p>
        <div v-if="detail.parts.length === 0" class="text-dim" style="padding: 20px; text-align: center">未参加任何答题</div>
        <div v-for="p in detail.parts" :key="p.quiz_id" class="part-row">
          <div style="flex: 1; min-width: 0">
            <div style="font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ p.title }}</div>
            <div class="text-dim" style="font-size: 12px">{{ fmtTime(p.joined_at) }} · {{ statusText(p.status) }}</div>
          </div>
          <div style="text-align: right">
            <div><b style="color: var(--warn)">{{ p.score }}</b> 分 · 第 <b>{{ p.rank }}</b> 名</div>
            <div class="text-dim" style="font-size: 12px">对 {{ p.correct_count }} / 错 {{ p.wrong_count }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { http, unwrap, LS } from '../api'

interface UserRow {
  id: number
  username: string
  nickname: string
  created_at: string
  quiz_count: number
  total_score: number
  correct_cnt: number
  wrong_cnt: number
  answer_cnt: number
  last_joined?: string
}

interface PartRow {
  quiz_id: number
  title: string
  status: string
  score: number
  correct_count: number
  wrong_count: number
  rank: number
  joined_at: string
}

const users = ref<UserRow[]>([])
const loading = ref(true)
const keyword = ref('')
const editing = ref(false)
const editTarget = ref<UserRow | null>(null)
const editForm = reactive({ username: '', nickname: '', password: '' })
const editErr = ref('')
const detail = ref<{ user: { id: number; nickname: string }; parts: PartRow[] } | null>(null)

let debounceTimer: number | null = null

onMounted(load)

async function load() {
  loading.value = true
  try {
    const kw = keyword.value.trim()
    const r = await http.get('/api/admin/users', {
      params: kw ? { keyword: kw } : {},
      headers: { Authorization: `Bearer ${localStorage.getItem(LS.adminToken)}` },
    })
    users.value = unwrap<UserRow[]>(r as never)
  } finally {
    loading.value = false
  }
}

function debouncedLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = window.setTimeout(load, 300)
}

function correctRate(u: UserRow) {
  if (!u.answer_cnt) return 0
  return Math.round((u.correct_cnt / u.answer_cnt) * 100)
}

function fmtTime(t?: string) {
  if (!t) return '—'
  return t.replace('T', ' ').replace(/\..*$/, '').replace(/Z$/, '')
}

function statusText(s: string) {
  return (
    {
      WAITING: '未开始',
      RUNNING: '进行中',
      ANSWERING: '答题中',
      PAUSED: '已暂停',
      RUSHING: '抢答中',
      REVEALING: '公布答案',
      FINISHED: '已结束',
    } as Record<string, string>
  )[s] || s
}

function openCreate() {
  editTarget.value = null
  editForm.username = ''
  editForm.nickname = ''
  editForm.password = ''
  editErr.value = ''
  editing.value = true
}

function openEdit(u: UserRow) {
  editTarget.value = u
  editForm.username = u.username
  editForm.nickname = u.nickname
  editForm.password = ''
  editErr.value = ''
  editing.value = true
}

async function saveUser() {
  editErr.value = ''
  const h = { headers: { Authorization: `Bearer ${localStorage.getItem(LS.adminToken)}` } }
  try {
    if (editTarget.value) {
      await http.put(
        `/api/admin/users/${editTarget.value.id}`,
        { nickname: editForm.nickname, password: editForm.password },
        h
      )
    } else {
      if (!editForm.username || !editForm.password || !editForm.nickname) {
        editErr.value = '用户名/密码/昵称均必填'
        return
      }
      await http.post('/api/admin/users', { ...editForm }, h)
    }
    editing.value = false
    load()
  } catch (e: any) {
    editErr.value = e?.response?.data?.msg || '保存失败'
  }
}

async function openDetail(u: UserRow) {
  const r = await http.get(`/api/admin/users/${u.id}`, {
    headers: { Authorization: `Bearer ${localStorage.getItem(LS.adminToken)}` },
  })
  detail.value = unwrap(r as never)
}

async function delUser(u: UserRow) {
  if (!confirm(`确定删除用户「${u.nickname}」？其所有参与记录、答题记录、成绩将一并删除`)) return
  await http.delete(`/api/admin/users/${u.id}`, {
    headers: { Authorization: `Bearer ${localStorage.getItem(LS.adminToken)}` },
  })
  load()
}
</script>

<style scoped>
.utable {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
  min-width: 900px;
}
.utable th {
  text-align: left;
  color: var(--text-dim);
  font-weight: 600;
  font-size: 12px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.utable td {
  padding: 11px 12px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.utable tr:hover td {
  background: rgba(108, 123, 255, 0.05);
}
.modal {
  position: fixed;
  inset: 0;
  background: rgba(5, 8, 18, 0.72);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  z-index: 50;
}
.modal-body {
  width: 460px;
  max-width: 100%;
}
.part-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  margin-bottom: 8px;
  background: var(--bg-soft);
}
.flbl {
  display: block;
  font-size: 12px;
  color: var(--text-dim);
  margin-bottom: 6px;
}
</style>
