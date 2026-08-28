<template>
  <div class="join-wrap">
    <header class="join-head">
      <div>
        <h1>选择答题活动</h1>
        <p class="sub">挑选一场活动开始作答</p>
      </div>
      <div class="account-box">
        <button class="account" @click="$router.push('/login')">
          <span class="avatar">{{ nick.slice(0, 1) }}</span>
          <span class="nick">{{ nick }}</span>
        </button>
        <button class="logout" @click="logout" title="退出登录">退出</button>
      </div>
    </header>

    <!-- 进入中 -->
    <div v-if="joining" class="card joining">
      <div class="spinner"></div>
      <p>正在进入答题…</p>
    </div>

    <template v-else>
      <p v-if="err" class="err">{{ err }}</p>

      <!-- 深链 /join/<id>：确认进入 -->
      <div v-if="linkBrief" class="card hero" @click="go(linkBrief.code)">
        <span class="hero-tag">受邀活动</span>
        <h2>{{ linkBrief.title }}</h2>
        <p class="desc">{{ linkBrief.description || '' }}</p>
        <button class="btn btn-primary go">进入答题</button>
      </div>

      <!-- 我参加过的比赛（含已结束） -->
      <template v-if="!linkBrief && mine.length">
        <div class="mine-title">我参加过的比赛</div>
        <div v-for="m in mine" :key="m.quiz_id" class="quiz-row" @click="goMine(m)">
          <div class="row-main">
            <div class="row-title">{{ m.title }}</div>
            <div class="row-meta">
              <span class="m-item">{{ m.code }}</span>
              <span class="dot"></span>
              <span>{{ statusText(m.status) }}</span>
              <span class="dot"></span>
              <span>我的 {{ m.score }} 分（对{{ m.correct }}/错{{ m.wrong }}）</span>
            </div>
          </div>
          <span class="chev">{{ m.status === 'FINISHED' ? '成绩' : '进入' }} ›</span>
        </div>
      </template>

      <!-- 活动列表（可加入 = WAITING） -->
      <template v-if="!linkBrief">
        <p v-if="loadingList" class="empty">加载中…</p>
        <template v-else-if="quizzes.length">
          <div v-for="q in quizzes" :key="q.id" class="quiz-row" @click="go(q.code)">
            <div class="row-main">
              <div class="row-title">{{ q.title }} <span v-if="q.joined" class="tag" style="margin-left:6px;font-size:12px">已加入</span></div>
              <div class="row-meta">
                <span class="m-item">{{ q.code }}</span>
                <span class="dot"></span>
                <span>{{ modeText(q.mode) }}</span>
                <span class="dot"></span>
                <span>{{ q.participant_count }} 人已加入</span>
              </div>
            </div>
            <span class="chev">›</span>
          </div>
        </template>
        <div v-else-if="!mine.length" class="card empty-card">
          <p>暂无可加入的答题活动</p>
          <p class="empty-sub">请等待管理员创建并开启</p>
        </div>
        <p v-else class="empty">暂无可加入的新活动</p>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userApi, globalToken } from '../api/user'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()

const joining = ref(false)
const loadingList = ref(true)
const err = ref('')
const quizzes = ref<{ id: number; code: string; title: string; description: string; mode: string; participant_count: number; joined: boolean }[]>([])
const mine = ref<{ quiz_id: number; code: string; title: string; status: string; mode: string; score: number; correct: number; wrong: number; joined_at: string; participant_count: number }[]>([])

const STATUS: Record<string, string> = {
  WAITING: '未开始', RUNNING: '进行中', PAUSED: '已暂停', RUSHING: '抢答中',
  ANSWERING: '答题中', REVEALING: '公布中', FINISHED: '已结束',
}
const statusText = (st: string) => STATUS[st] || st
const modeText = (m: string) => (m === 'rush' ? '抢答模式' : m === 'exam' ? '考试模式' : '普通模式')

/** 已结束 → 回看成绩/排行；进行中 → 回到答题页（考试模式走 /exam） */
function examPath(m: { code: string; mode?: string; status: string }): string | null {
  return m.mode === 'exam' ? `/exam/${m.code}` : null
}

async function goMine(m: { code: string; mode?: string; status: string }) {
  joining.value = true
  err.value = ''
  // 先同步开新 tab（在点击手势内，避免 await 后被浏览器拦截），加入完成后再导航
  const win = window.open('about:blank')
  try {
    // 已结束且无 token：重新 join 换 token（老参与者可换）再看成绩
    if (!localStorage.getItem(LS.userToken(m.code))) {
      const { token, quiz, user } = await userApi.joinQuiz(m.code)
      localStorage.setItem(LS.userToken(quiz.code), token)
      localStorage.setItem(LS.userId(quiz.code), String(user.id))
      localStorage.setItem(LS.nickname(quiz.code), user.nickname)
    }
    const exam = examPath({ code: m.code, mode: m.mode, status: m.status })
    const path = exam ?? (m.status === 'FINISHED' ? `/rank/${m.code}` : `/quiz/${m.code}`)
    if (win) win.location.href = path
    else router.replace(path) // 弹窗被拦截时兜底当前页
  } catch (e: any) {
    win?.close()
    err.value = e?.response?.data?.msg || '进入失败'
  } finally {
    joining.value = false
  }
}
const linkBrief = ref<{ id: number; code: string; title: string; description: string } | null>(null)
const nick = computed(() => localStorage.getItem(LS.userNick) || '已登录')

const linkQuizId = computed(() => String(route.params.id || ''))

onMounted(async () => {
  // 未登录 → 登录后回来
  if (!globalToken()) {
    router.replace({
      path: '/login',
      query: linkQuizId.value ? { redirect: `/join/${linkQuizId.value}` } : {},
    })
    return
  }
  if (linkQuizId.value) {
    // 深链：展示该活动并自动进入
    try {
      linkBrief.value = await userApi.quizBrief(linkQuizId.value)
      go(linkQuizId.value, false) // 非点击手势触发，自动进入仅当前页（新开会被拦截）
    } catch {
      err.value = '答题活动不存在'
    }
  } else {
    // 活动列表 + 我参加过的
    try {
      const [r, mineR] = await Promise.all([userApi.quizList(), userApi.myQuizzes()])
      quizzes.value = r.items
      // WAITING 的比赛已在「可加入」区（带已加入标记），不重复展示
      mine.value = mineR.items.filter(m => m.status !== 'WAITING')
    } catch {
      err.value = '活动列表加载失败，请刷新重试'
    } finally {
      loadingList.value = false
    }
  }
})

function logout() {
  Object.keys(localStorage)
    .filter(k => k.startsWith('kaoshi_token_') || k === LS.userGlobalToken || k === LS.userNick)
    .forEach(k => localStorage.removeItem(k))
  router.replace('/login')
}

async function go(quizId: string, newTab = true) {
  if (!quizId) return
  joining.value = true
  err.value = ''
  // 先同步开新 tab（在点击手势内，避免 await 后被浏览器拦截），加入完成后再导航
  const win = newTab ? window.open('about:blank') : null
  try {
    let path: string
    // 已有答题 token 直接进（brief 补查模式，考试走 /exam）
    if (localStorage.getItem(LS.userToken(quizId))) {
      const brief = await userApi.quizBrief(quizId).catch(() => null)
      path = brief?.mode === 'exam' ? `/exam/${quizId}` : `/quiz/${quizId}`
    } else {
      // 加入换取答题 token
      const { token, quiz, user } = await userApi.joinQuiz(quizId)
      localStorage.setItem(LS.userToken(quiz.code), token)
      localStorage.setItem(LS.userId(quiz.code), String(user.id))
      localStorage.setItem(LS.nickname(quiz.code), user.nickname)
      path = quiz.mode === 'exam' ? `/exam/${quiz.code}` : `/quiz/${quiz.code}`
    }
    if (win) win.location.href = path
    else router.replace(path) // 弹窗被拦截或非点击手势时兜底当前页
  } catch (e: any) {
    win?.close()
    err.value = e?.response?.data?.msg || '加入失败'
  } finally {
    joining.value = false
  }
}
</script>

<style scoped>
.mine-title {
  font-size: 13px;
  letter-spacing: 1px;
  opacity: 0.75;
  margin: 18px 2px 8px;
}

.join-wrap {
  max-width: 560px;
  margin: 0 auto;
  padding: 8vh 16px 48px;
}
.join-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 28px;
}
h1 {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.sub {
  margin-top: 6px;
  color: var(--text-dim);
  font-size: 15px;
}
.account-box {
  display: flex;
  align-items: center;
  gap: 8px;
}
.logout {
  border: 1px solid var(--border);
  background: var(--card);
  color: var(--text-dim);
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: var(--shadow);
  font-family: inherit;
}
.logout:hover {
  color: var(--danger);
  border-color: var(--danger);
}
.account {
  display: flex;
  align-items: center;
  gap: 8px;
  border: none;
  background: var(--card);
  border-radius: 999px;
  padding: 5px 12px 5px 5px;
  cursor: pointer;
  box-shadow: var(--shadow);
  font-family: inherit;
}
.avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--primary);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.nick {
  font-size: 13px;
  font-weight: 600;
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.err {
  color: var(--danger);
  font-size: 14px;
  margin-bottom: 16px;
}
.joining {
  text-align: center;
  padding: 56px 20px;
  color: var(--text-dim);
}
.spinner {
  width: 26px;
  height: 26px;
  margin: 0 auto 14px;
  border-radius: 50%;
  border: 3px solid var(--card-2);
  border-top-color: var(--primary);
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.hero {
  text-align: center;
  padding: 36px 24px;
  cursor: pointer;
}
.hero-tag {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
  background: rgba(0, 113, 227, 0.1);
  border-radius: 999px;
  padding: 4px 12px;
  margin-bottom: 14px;
}
.hero h2 {
  font-size: 22px;
  font-weight: 700;
}
.hero .desc {
  color: var(--text-dim);
  font-size: 14px;
  margin: 8px 0 22px;
}
.go {
  min-width: 200px;
}
.quiz-row {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 16px 18px;
  margin-bottom: 10px;
  cursor: pointer;
  box-shadow: var(--shadow);
  transition: transform 0.12s ease;
}
.quiz-row:hover {
  transform: translateY(-1px);
}
.quiz-row:active {
  transform: scale(0.985);
}
.row-main {
  flex: 1;
  min-width: 0;
}
.row-title {
  font-size: 16px;
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 5px;
  font-size: 12px;
  color: var(--text-dim);
}
.dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--text-dim);
  opacity: 0.6;
}
.chev {
  font-size: 24px;
  color: var(--text-dim);
  font-weight: 300;
  line-height: 1;
}
.empty-card {
  text-align: center;
  padding: 48px 20px;
}
.empty-sub {
  color: var(--text-dim);
  font-size: 13px;
  margin-top: 6px;
}
</style>
