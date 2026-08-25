<template>
  <div class="page console-page">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px">
      <button class="btn btn-ghost" style="padding: 8px 14px" @click="$router.push(`/admin/quiz/${quizId}`)">←</button>
      <h1 style="flex: 1; font-size: 20px">{{ quiz?.title || '控制台' }}</h1>
      <button v-if="quiz" class="tag" style="cursor: pointer; border: 1px dashed var(--border)" title="点击复制加入链接" @click="copyLink">🔗 {{ joinLink }}</button>
      <span class="tag" :class="'st-' + status">{{ statusText }}</span>
    </div>

    <div class="layout">
      <!-- 左：题目列表 -->
      <div class="card panel">
        <h3 class="panel-title">题目列表</h3>
        <div
          v-for="(q, i) in questions"
          :key="q.id"
          class="q-item"
          :class="{ cur: i === curIndex }"
        >
          <span class="q-no">{{ String(i + 1).padStart(2, '0') }}</span>
          <span>{{ typeText(q.type) }}</span>
          <span class="text-dim" style="font-size: 12px">{{ q.score }}分</span>
          <span v-if="i === curIndex" style="color: var(--primary); font-size: 12px">● 当前</span>
        </div>
        <p v-if="questions.length === 0" class="text-dim">暂无题目</p>
      </div>

      <!-- 中：当前题 -->
      <div class="card panel mid">
        <div v-if="!curQuestion" class="text-dim" style="text-align: center; padding: 60px 0">
          {{ status === 'WAITING' ? '等待开始，点击底部「开始答题」发布第 1 题' : '等待发布题目' }}
        </div>
        <template v-else>
          <div class="q-meta">
            <span class="tag">第 {{ curIndex + 1 }} / {{ questions.length }} 题</span>
            <span class="tag">{{ typeText(curQuestion.type) }}</span>
            <span class="tag">{{ curQuestion.score }} 分</span>
            <span v-if="remainSec > 0" class="tag" style="color: var(--warn)">⏱ {{ remainSec }}s</span>
          </div>
          <h2 class="q-content">{{ curQuestion.content }}</h2>
          <div class="opts-preview">
            <div
              v-for="o in curQuestion.options"
              :key="o.label"
              class="opt-line"
              :class="{ correct: revealed && curQuestion.answer?.includes(o.label) }"
            >
              <b>{{ o.label }}.</b> {{ o.content }}
              <span v-if="distribution[o.label]" class="dist-chip">{{ distribution[o.label] }}人</span>
            </div>
          </div>
        </template>
      </div>

      <!-- 右：实时数据 -->
      <div class="card panel">
        <!-- 抢答获答者 -->
        <div v-if="rushWinners.length > 0" class="rush-box">
          <h3 class="panel-title" style="color: var(--danger)">⚡ 抢答获答者</h3>
          <div v-for="w in rushWinners" :key="w.user_id" class="rush-winner-row">
            <span class="rk" :class="'top' + Math.min(w.rank, 3)">{{ w.rank }}</span>
            <span style="flex: 1">{{ w.nickname }}</span>
            <span class="text-dim" style="font-size: 12px">+{{ w.bonus }}分</span>
          </div>
        </div>

        <h3 class="panel-title">实时数据</h3>
        <div class="stat-grid">
          <div class="stat"><div class="n">{{ stats.participants }}</div><div class="l">参与人数</div></div>
          <div class="stat"><div class="n">{{ stats.answered }}</div><div class="l">已答</div></div>
          <div class="stat"><div class="n">{{ stats.participants - stats.answered }}</div><div class="l">未答</div></div>
          <div class="stat"><div class="n" style="color: var(--success)">{{ stats.correct }}</div><div class="l">正确</div></div>
          <div class="stat"><div class="n" style="color: var(--danger)">{{ stats.wrong }}</div><div class="l">错误</div></div>
          <div class="stat"><div class="n" style="color: var(--warn)">{{ stats.max_score }}</div><div class="l">最高分</div></div>
        </div>

        <h3 class="panel-title" style="margin-top: 18px">答案分布</h3>
        <div v-if="Object.keys(distribution).length === 0" class="text-dim" style="font-size: 13px">暂无提交</div>
        <div v-for="(cnt, ans) in distribution" :key="ans" class="dist-row">
          <span class="dist-label">{{ ans === '-' ? '未答' : ans }}</span>
          <div class="dist-bar-wrap"><div class="dist-bar" :style="{ width: barWidth(cnt) }"></div></div>
          <span class="text-dim" style="font-size: 12px">{{ cnt }}</span>
        </div>
      </div>
    </div>

    <!-- 底部控制 -->
    <div class="card controls">
      <button class="btn btn-ghost" :disabled="curIndex <= 0 || status === 'WAITING'" @click="ctrl('previous')">← 上一题</button>
      <button v-if="status === 'WAITING'" class="btn btn-primary big" @click="ctrl('start')">▶ 开始答题</button>
      <button v-if="status === 'PAUSED'" class="btn btn-primary big" @click="ctrl('resume')">▶ 继续</button>
      <button v-if="canRush" class="btn big" style="background: linear-gradient(135deg, #ff7062, #e0404f)" @click="ctrl('rush/start')">⚡ 开始抢答</button>
      <button v-if="status === 'RUSHING'" class="btn btn-ghost" style="color: var(--warn)" @click="ctrl('rush/end')">■ 结束抢答</button>
      <button v-if="status === 'ANSWERING' || status === 'REVEALING'" class="btn btn-ghost" style="color: var(--warn)" @click="ctrl('pause')">⏸ 暂停</button>
      <button v-if="canReveal" class="btn btn-ghost" style="color: var(--success)" @click="ctrl('reveal')">📢 公布答案</button>
      <button v-if="status !== 'WAITING' && status !== 'FINISHED'" class="btn btn-primary big" @click="ctrl('next')">下一题 →</button>
      <button v-if="status !== 'WAITING' && status !== 'FINISHED'" class="btn btn-danger" @click="ctrl('end')">■ 结束答题</button>
      <button v-if="status !== 'WAITING'" class="btn btn-danger" @click="ctrl('reset')" title="清空答题/抢答记录与成绩，活动回到未开始">↺ 重置比赛</button>
      <span v-if="status === 'FINISHED'" class="text-dim" style="padding: 10px">答题已结束</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { adminApi, type Quiz, type Question } from '../api/admin'
import { adminToken } from '../api/admin'
import { QuizWS } from '../ws'
import { Ev, type WSMessage } from '../ws/types'
import { LS } from '../api'

const route = useRoute()
const quizId = Number(route.params.id)

const quiz = ref<Quiz | null>(null)
const questions = ref<Question[]>([])
const status = ref('WAITING')
const curIndex = ref(-1)
const curQuestion = ref<Question | null>(null)
const revealed = ref(false)
const deadlineAt = ref(0)
const remainMs = ref(0)

const stats = reactive({ participants: 0, answered: 0, correct: 0, wrong: 0, max_score: 0 })
const distribution = ref<Record<string, number>>({})

let ws: QuizWS | null = null
let tickTimer: number | null = null

const remainSec = computed(() => Math.ceil(remainMs.value / 1000))
const canReveal = computed(() => curIndex.value >= 0 && (status.value === 'ANSWERING' || status.value === 'REVEALING'))
const rushWinners = ref<Array<{ user_id: number; nickname: string; rank: number; bonus: number }>>([])
const canRush = computed(
  () =>
    !!quiz.value?.rush_enabled &&
    status.value === 'ANSWERING' &&
    curIndex.value >= 0 &&
    (rushWinners.value?.length ?? 0) === 0
)
const statusText = computed(() => ({ WAITING: '未开始', RUNNING: '进行中', PAUSED: '已暂停', RUSHING: '抢答中', ANSWERING: '答题中', REVEALING: '公布答案', FINISHED: '已结束' } as Record<string, string>)[status.value] || status.value)

onMounted(async () => {
  const { quiz: q } = await adminApi.getQuiz(quizId)
  quiz.value = q
  questions.value = await adminApi.listQuestions(quizId)
  status.value = q.status

  const token = adminToken()
  if (!token) {
    location.href = '/admin/login'
    return
  }
  ws = new QuizWS({
    token,
    quiz: quizId,
    onStatus: () => {},
    onEvent: (msg: WSMessage) => handleEvent(msg),
  })

  tickTimer = window.setInterval(() => {
    if (deadlineAt.value > 0) {
      remainMs.value = Math.max(0, deadlineAt.value - Date.now())
    } else {
      remainMs.value = 0
    }
  }, 200)
})

onUnmounted(() => {
  ws?.close()
  if (tickTimer) clearInterval(tickTimer)
})

function handleEvent(msg: WSMessage) {
  const d = msg.data || {}
  switch (msg.event) {
    case Ev.Sync:
      status.value = d.status || 'WAITING'
      rushWinners.value = d.rush_winners || []
      if (d.question) {
        applyQuestion(d.question, d.deadline_at || 0)
      }
      break
    case Ev.QuestionPublish:
      status.value = d.status || 'ANSWERING'
      revealed.value = false
      distribution.value = {}
      rushWinners.value = []
      stats.answered = 0
      stats.correct = 0
      stats.wrong = 0
      if (d.question) applyQuestion(d.question, d.deadline_at || 0)
      break
    case Ev.QuestionCountdown:
      if (d.deadline_at) deadlineAt.value = d.deadline_at
      if (d.remain_sec === 0) deadlineAt.value = 0
      break
    case Ev.StatisticsUpdate:
      stats.participants = d.participants_all ?? d.participants ?? stats.participants
      stats.answered = d.answered ?? 0
      stats.correct = d.correct ?? 0
      stats.wrong = d.wrong ?? 0
      stats.max_score = d.max_score ?? stats.max_score
      distribution.value = d.distribution || {}
      break
    case Ev.AnswerReveal:
      revealed.value = true
      status.value = 'REVEALING'
      if (d.distribution) distribution.value = d.distribution
      if (d.stats) {
        stats.answered = d.stats.total
        stats.correct = d.stats.correct
        stats.wrong = d.stats.wrong
      }
      break
    case Ev.ActivityStart:
      status.value = 'ANSWERING'
      break
    case Ev.ActivityPause:
      status.value = 'PAUSED'
      break
    case Ev.ActivityResume:
      status.value = 'ANSWERING'
      break
    case Ev.RushStart:
      status.value = 'RUSHING'
      rushWinners.value = []
      break
    case Ev.RushEnd:
      status.value = 'ANSWERING'
      rushWinners.value = d.winners || []
      if (d.answer_deadline_at) deadlineAt.value = d.answer_deadline_at
      break
    case Ev.ActivityEnd:
      status.value = 'FINISHED'
      break
  }
}

function applyQuestion(q: any, deadline: number) {
  curIndex.value = (q.index || 1) - 1
  // 管理端题目信息从题目列表取（含答案）
  const local = questions.value[curIndex.value]
  if (local && local.id === q.id) {
    curQuestion.value = local
  } else {
    curQuestion.value = { ...q, analysis: '', answer: '', options: q.options || [] } as Question
  }
  deadlineAt.value = deadline
  remainMs.value = deadline ? Math.max(0, deadline - Date.now()) : 0
}

async function ctrl(action: string) {
  if (action === 'end' && !confirm('确定结束答题？将生成最终成绩与排行榜')) return
  if (action === 'reset' && !confirm('确定重置？将清空所有答题/抢答记录与成绩，比赛回到未开始状态（题目和已加入的选手保留）')) return
  try {
    await fetch(`/api/admin/quiz/${quizId}/${action}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${localStorage.getItem(LS.adminToken)}` },
    })
  } catch {
    /* WS 会同步状态 */
  }
}

function typeText(t: string) {
  return ({ single: '单选', multiple: '多选', judge: '判断' } as Record<string, string>)[t] || t
}

const joinLink = computed(() => (quiz.value ? `${location.origin}/join/${quiz.value.id}` : ''))

async function copyLink() {
  try {
    await navigator.clipboard.writeText(joinLink.value)
    alert('已复制加入链接：\n' + joinLink.value)
  } catch {
    alert('加入链接：' + joinLink.value)
  }
}

function barWidth(cnt: number) {
  const total = Object.values(distribution.value).reduce((a, b) => a + b, 0) || 1
  return `${Math.max(4, Math.round((cnt / total) * 100))}%`
}
</script>

<style scoped>
.console-page {
  max-width: 1280px;
}
.layout {
  display: grid;
  grid-template-columns: 220px 1fr 300px;
  gap: 14px;
  align-items: start;
}
.panel {
  min-height: 380px;
}
.panel-title {
  font-size: 14px;
  color: var(--text-dim);
  margin-bottom: 12px;
}
.q-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  margin-bottom: 6px;
  font-size: 14px;
  border: 1px solid transparent;
}
.q-item.cur {
  background: rgba(108, 123, 255, 0.12);
  border-color: var(--primary);
}
.q-no {
  font-weight: 800;
  color: var(--primary);
}
.q-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.q-content {
  font-size: 18px;
  line-height: 1.5;
  margin-bottom: 18px;
}
.opts-preview {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.opt-line {
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--bg-soft);
  border: 1px solid var(--border);
  font-size: 15px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.opt-line.correct {
  border-color: var(--success);
  background: rgba(46, 204, 143, 0.12);
}
.dist-chip {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-dim);
}
.rush-box {
  background: rgba(255, 93, 108, 0.08);
  border: 1px solid var(--danger);
  border-radius: 10px;
  padding: 10px 12px;
  margin-bottom: 14px;
}
.rush-winner-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 4px;
  font-size: 14px;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.stat {
  background: var(--bg-soft);
  border-radius: 10px;
  padding: 10px;
  text-align: center;
}
.stat .n {
  font-size: 20px;
  font-weight: 800;
}
.stat .l {
  font-size: 11px;
  color: var(--text-dim);
}
.dist-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.dist-label {
  width: 44px;
  font-size: 13px;
  font-weight: 700;
  text-align: center;
}
.dist-bar-wrap {
  flex: 1;
  height: 14px;
  border-radius: 7px;
  background: var(--bg-soft);
  overflow: hidden;
}
.dist-bar {
  height: 100%;
  border-radius: 7px;
  background: linear-gradient(90deg, var(--primary), var(--primary-strong));
  transition: width 0.3s ease;
}
.controls {
  margin-top: 14px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
  position: sticky;
  bottom: 8px;
  z-index: 10;
}
.big {
  flex: 1;
  min-width: 140px;
}
.st-WAITING { color: var(--text-dim); }
.st-ANSWERING { color: var(--success); }
.st-RUNNING { color: var(--success); }
.st-PAUSED { color: var(--warn); }
.st-FINISHED { color: var(--text-dim); }

@media (max-width: 960px) {
  .layout {
    grid-template-columns: 1fr;
  }
}
</style>
