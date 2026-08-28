<template>
  <div class="page console-page">
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px">
      <button class="btn btn-ghost" style="padding: 8px 14px" @click="$router.push(`/admin/quiz/${quizId}`)">←</button>
      <h1 style="flex: 1; font-size: 20px">{{ quiz?.title || '控制台' }}</h1>
      <button v-if="quiz" class="tag" style="cursor: pointer; border: 1px dashed var(--border)" title="点击复制加入链接" @click="copyLink">🔗 {{ joinLink }}</button>
      <span class="tag" :class="'st-' + status">{{ statusText }}</span>
      <button class="tag" style="cursor: pointer" title="大屏排行榜（新标签页打开）" @click="openRank">🏆 排行榜</button>
    </div>

    <div class="layout" :class="{ 'with-right': isExam }">
      <!-- 左：题目列表（视口内滚动，自动跟随当前题） -->
      <div class="card panel q-panel">
        <div class="q-head">
          <h3 class="panel-title" style="margin-bottom:0">题目列表</h3>
          <span class="q-count">{{ questions.length }}</span>
        </div>
        <div class="q-scroll" ref="qScroll">
          <div
            v-for="(q, i) in questions"
            :key="q.id"
            class="q-item"
            :class="{ cur: i === curIndex, done: i < curIndex, rush: !q.required }"
            :ref="(el) => { if (i === curIndex) curEl = el as HTMLElement }"
          >
            <span class="q-mode">{{ q.required ? '必' : '抢' }}</span>
            <span class="q-no">{{ String(i + 1).padStart(2, '0') }}</span>
            <span class="text-dim" style="font-size: 11px; margin-left: auto">{{ q.score }}分</span>
          </div>
          <p v-if="questions.length === 0" class="text-dim">暂无题目</p>
        </div>
        <div class="q-foot text-dim" v-if="curIndex >= 0">
          <div class="q-progress"><div class="q-progress-fill" :style="{ width: ((curIndex + 1) / questions.length) * 100 + '%' }"></div></div>
          <span style="font-size: 11px">{{ curIndex + 1 }} / {{ questions.length }}</span>
        </div>
      </div>

      <!-- 中：当前题 -->
      <div class="card panel mid">
        <div v-if="isExam && (status === 'RUNNING' || status === 'FINISHED')" class="exam-live">
          <div class="el-head">
            <h3>{{ status === 'RUNNING' ? '实时作答进度' : '全卷作答汇总' }}</h3>
            <span class="text-dim" style="font-size: 12px">选择即自动保存 · 到时或「结束考试」统一收卷计分</span>
          </div>
          <div class="el-grid">
            <div v-for="q in ov.questions" :key="q.question_id" class="el-cell" :title="q.content">
              <div class="el-top">
                <span class="el-no">{{ String(q.index).padStart(2, '0') }}</span>
                <span class="el-rate" :class="q.correct_rate >= 60 ? 'ok' : q.correct_rate > 0 ? 'low' : ''">{{ q.correct_rate.toFixed(0) }}%</span>
              </div>
              <div class="el-bar"><div class="el-fill" :style="{ width: elPct(q) }"></div></div>
              <div class="el-meta"><b>{{ q.answered }}</b> / {{ ov.participants }} 人</div>
            </div>
            <p v-if="!ov.questions.length" class="text-dim" style="font-size: 12px; grid-column: 1 / -1; text-align: center; padding: 20px 0">暂无数据，等选手开始作答后每 5 秒刷新</p>
          </div>
        </div>
        <div v-else-if="!curQuestion" class="text-dim" style="text-align: center; padding: 60px 0">
          {{ status === 'WAITING' ? (isExam ? '等待开始，点击底部「开始考试」下发全卷' : '等待开始，点击底部「开始答题」发布第 1 题') : '等待发布题目' }}
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

          <!-- 必答题统计条（大屏投影） -->
          <div v-if="curQuestion?.required && rushWinners.length === 0" class="rush-result">
            <div class="rr-hero">
              <span class="rr-bolt">📝</span>
              <div class="rr-hero-text">
                <b>第 {{ curIndex + 1 }} 题 · 必答题</b>
                <small>共 {{ stats.participants }} 人</small>
              </div>
            </div>
            <div class="rr-sec">
              <small>已答</small>
              <b class="rr-ans">{{ stats.answered }}人</b>
            </div>
            <div class="rr-sec">
              <small>答对</small>
              <b class="rr-ans ok">{{ stats.correct }}人</b>
            </div>
            <div class="rr-sec">
              <small>答错</small>
              <b class="rr-ans no">{{ stats.wrong }}人</b>
            </div>
            <div v-if="revealed" class="rr-sec">
              <small>正确答案</small>
              <b class="rr-ans ok">{{ curQuestion.answer }}</b>
            </div>
          </div>

          <!-- 抢答结果条（大屏投影） -->
          <div v-if="rushWinners.length > 0" class="rush-result">
            <div class="rr-hero">
              <span class="rr-bolt">⚡</span>
              <div class="rr-hero-text">
                <b>{{ rushWinners.map((w) => w.nickname).join('、') }}</b>
                <small>抢到了</small>
              </div>
            </div>
            <div class="rr-sec">
              <small>回答答案</small>
              <b class="rr-ans" :class="revealed ? (winnerCorrect ? 'ok' : 'no') : ''">{{ winnerAnswer || '作答中…' }}</b>
            </div>
            <div v-if="revealed" class="rr-sec">
              <small>正确答案</small>
              <b class="rr-ans ok">{{ curQuestion.answer }}</b>
            </div>
            <div v-if="revealed" class="rr-sec">
              <small>答题得分</small>
              <b class="rr-ans" :class="winnerCorrect ? 'ok' : 'no'">{{ (winnerScore > 0 ? '+' : '') + winnerScore + '分' }}</b>
            </div>
          </div>
        </template>
      </div>

      <!-- 右：实时概况（考试/自由切题模式） -->
      <div v-if="isExam" class="card panel right-ov">
        <h3 class="panel-title">实时概况</h3>
        <div class="ov-grid">
          <div class="ov-cell"><b>{{ ov.participants }}</b><small>参赛人数</small></div>
          <div class="ov-cell"><b>{{ ov.finished }}</b><small>已交卷</small></div>
          <div class="ov-cell ok"><b>{{ ov.max_score }}</b><small>最高分</small></div>
          <div class="ov-cell"><b>{{ ov.min_score }}</b><small>最低分</small></div>
          <div class="ov-cell"><b>{{ ov.avg_score.toFixed(1) }}</b><small>平均分</small></div>
          <div class="ov-cell"><b>{{ ov.avg_correct_rate.toFixed(0) }}%</b><small>平均正确率</small></div>
        </div>
        <h3 class="panel-title" style="margin-top: 16px">前 10 名</h3>
        <div class="ov-top">
          <div v-for="r in ov.ranking.slice(0, 10)" :key="r.user_id" class="ov-row">
            <span class="ov-rk" :class="'p' + Math.min(r.rank, 3)">{{ r.rank }}</span>
            <span class="ov-nm"><b>{{ r.nickname }}</b><small v-if="r.submitted_at">{{ fmtT(r.submitted_at) }} 交卷</small></span>
            <span class="ov-sc">{{ r.score }}<em>分</em></span>
          </div>
          <p v-if="!ov.ranking.length" class="text-dim" style="font-size: 12px; padding: 8px 0">暂无数据</p>
        </div>
        <div class="text-dim" style="font-size: 11px; margin-top: 12px">
          每 5 秒自动刷新 ·
          <a href="javascript:;" style="color: var(--primary)" @click="$router.push(`/admin/quiz/${quizId}/stats`)">详细统计 →</a>
        </div>
      </div>

    </div>

    <!-- 底部控制 -->
    <div class="card controls">
      <template v-if="!isExam">
        <button class="btn btn-ghost" :disabled="curIndex <= 0 || status === 'WAITING'" @click="ctrl('previous')">← 上一题</button>
      </template>
      <button v-if="status === 'WAITING'" class="btn btn-primary big" @click="ctrl('start')">▶ {{ isExam ? '开始考试' : '开始答题' }}</button>
      <template v-if="!isExam">
        <button v-if="status === 'PAUSED'" class="btn btn-primary big" @click="ctrl('resume')">▶ 继续</button>
        <button v-if="canRush" class="btn big" style="background: linear-gradient(135deg, #ff7062, #e0404f)" @click="ctrl('rush/start')">⚡ 开始抢答</button>
        <button v-if="status === 'RUSHING'" class="btn btn-ghost" style="color: var(--warn)" @click="ctrl('rush/end')">■ 结束抢答</button>
        <button v-if="curQuestion?.required && status === 'ANSWERING'" class="btn btn-ghost" style="color: var(--success)" @click="ctrl('reveal')">📢 显示答案</button>
        <button v-if="status === 'ANSWERING' || status === 'REVEALING'" class="btn btn-ghost" style="color: var(--warn)" @click="ctrl('pause')">⏸ 暂停</button>
      </template>
      <button v-if="!isExam && status !== 'WAITING' && status !== 'FINISHED'" class="btn btn-primary big" @click="ctrl('next')">下一题 →</button>
      <button v-if="isExam && status === 'RUNNING'" class="btn btn-danger big" @click="ctrl('end')">■ 结束考试（统一收卷）</button>
      <button v-if="status !== 'WAITING'" class="btn btn-danger" @click="ctrl('reset')" title="清空答题/抢答记录与成绩，活动回到未开始">↺ 重置比赛</button>
      <span v-if="status === 'FINISHED'" class="text-dim" style="padding: 10px">考试已结束</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi, type Quiz, type Question } from '../api/admin'
import { adminToken } from '../api/admin'
import { QuizWS } from '../ws'
import { Ev, type WSMessage } from '../ws/types'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()
const quizId = String(route.params.id || '')

const quiz = ref<Quiz | null>(null)
const questions = ref<Question[]>([])
const status = ref('WAITING')
const curIndex = ref(-1)
const qScroll = ref<HTMLElement | null>(null)
const curEl = ref<HTMLElement | null>(null)
const curQuestion = ref<Question | null>(null)
const revealed = ref(false)
const deadlineAt = ref(0)
const remainMs = ref(0)

const stats = reactive({ participants: 0, answered: 0, correct: 0, wrong: 0, max_score: 0 })
const distribution = ref<Record<string, number>>({})
const answerScores = ref<Record<string, number>>({})

let ws: QuizWS | null = null
let tickTimer: number | null = null

const remainSec = computed(() => Math.ceil(remainMs.value / 1000))
const rushWinners = ref<Array<{ user_id: number; nickname: string; rank: number; bonus: number }>>([])
const canRush = computed(
  () =>
    !!quiz.value?.rush_enabled &&
    status.value === 'ANSWERING' &&
    curIndex.value >= 0 &&
    !curQuestion.value?.required &&
    (rushWinners.value?.length ?? 0) === 0
)
/** 考试（自由切题）模式：无逐题控制 */
const isExam = computed(() => quiz.value?.mode === 'exam')
const statusText = computed(() => ({ WAITING: '未开始', RUNNING: '进行中', PAUSED: '已暂停', RUSHING: '抢答中', ANSWERING: '答题中', REVEALING: '公布答案', FINISHED: '已结束' } as Record<string, string>)[status.value] || status.value)

/** 获答者提交的答案：抢答题 distribution 的非“-”键即其作答 */
const winnerAnswer = computed(() => Object.keys(distribution.value).filter((k) => k !== '-').join(''))
const winnerCorrect = computed(() => !!winnerAnswer.value && winnerAnswer.value === curQuestion.value?.answer)
/** 服务端判定的实际得分（抢答答错可为负） */
const winnerScore = computed(() => answerScores.value[winnerAnswer.value] ?? 0)
watch(curIndex, () => {
  nextTick(() => {
    const box = qScroll.value, el = curEl.value
    if (!box || !el) return
    const top = el.offsetTop - box.clientHeight / 2 + el.offsetHeight / 2
    box.scrollTo({ top: Math.max(0, top), behavior: 'smooth' })
  })
})


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
      stats.participants = d.quiz?.participant_count ?? stats.participants
      // 刷新恢复：公布阶段带回答案分布与已公布标记
      if (d.status === 'REVEALING') {
        revealed.value = true
        if (d.distribution) distribution.value = d.distribution
        if (d.answer_scores) answerScores.value = d.answer_scores
      }
      if (d.question) {
        applyQuestion(d.question, d.deadline_at || 0)
      }
      break
    case Ev.QuestionPublish:
      status.value = d.status || 'ANSWERING'
      revealed.value = false
      distribution.value = {}
      answerScores.value = {}
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
      answerScores.value = d.answer_scores || {}
      break
    case Ev.AnswerReveal:
      revealed.value = true
      status.value = 'REVEALING'
      if (d.distribution) distribution.value = d.distribution
      if (d.answer_scores) answerScores.value = d.answer_scores
      if (d.stats) {
        stats.answered = d.stats.total
        stats.correct = d.stats.correct
        stats.wrong = d.stats.wrong
      }
      break
    case Ev.ActivityStart:
      // 考试模式开始后状态为 RUNNING（自由切题，无逐题发布）；普通模式随后发布第 1 题为 ANSWERING
      status.value = isExam.value ? 'RUNNING' : 'ANSWERING'
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

/** 考试模式右侧实时概况（轮询整场统计，5s；与统计页同源数据） */
const ov = reactive({
  participants: 0,
  finished: 0,
  max_score: 0,
  min_score: 0,
  avg_score: 0,
  avg_correct_rate: 0,
  ranking: [] as { rank: number; user_id: number; nickname: string; score: number; submitted_at: number }[],
  questions: [] as { index: number; question_id: number; content: string; answered: number; correct: number; correct_rate: number }[],
})
let ovTimer: number | null = null
async function loadOverview() {
  try {
    const s = await adminApi.statistics(quizId)
    ov.participants = s.participants
    ov.finished = s.finished
    ov.max_score = s.max_score
    ov.min_score = s.min_score
    ov.avg_score = s.avg_score
    ov.avg_correct_rate = s.avg_correct_rate
    ov.ranking = s.ranking || []
    ov.questions = s.questions || []
  } catch {
    /* 静默：下一轮重试 */
  }
}
watch(status, (s) => {
  if (!isExam.value) return
  if (s === 'RUNNING' || s === 'FINISHED') {
    loadOverview()
    if (!ovTimer) ovTimer = window.setInterval(loadOverview, 5000)
  } else if (ovTimer) {
    clearInterval(ovTimer)
    ovTimer = null
  }
})
onUnmounted(() => {
  if (ovTimer) clearInterval(ovTimer)
})

/** 新标签页打开大屏排行榜（admin 门禁） */
function openRank() {
  window.open(router.resolve(`/admin/rank/${quizId}`).href, '_blank')
}

/** 逐题作答进度条宽度 */
function elPct(q: { answered: number }) {
  if (!ov.participants) return '0%'
  return Math.min(100, (q.answered / ov.participants) * 100) + '%'
}

/** 交卷时间（HH:MM） */
function fmtT(ms?: number) {
  if (!ms) return ''
  return new Date(ms).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(joinLink.value)
    alert('已复制加入链接：\n' + joinLink.value)
  } catch {
    alert('加入链接：' + joinLink.value)
  }
}

</script>

<style scoped>
.console-page {
  max-width: 1280px;
}
.layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 14px;
  align-items: start;
}
.layout.with-right {
  grid-template-columns: 280px 1fr 280px;
}
/* 考试模式右侧实时概况栏 */
.right-ov {
  padding: 14px;
  position: sticky;
  top: 14px;
  max-height: calc(100vh - 200px);
  overflow-y: auto;
}
.ov-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
.ov-cell {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 10px 6px;
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.ov-cell b {
  font-size: 18px;
  font-weight: 800;
  color: var(--text);
}
.ov-cell small {
  font-size: 11px;
  color: var(--text-dim);
}
.ov-cell.ok b {
  color: var(--success);
}
.ov-top {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.ov-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 6px;
  border-radius: 8px;
  font-size: 13px;
}
.ov-row:hover {
  background: rgba(108, 123, 255, 0.06);
}
.ov-rk {
  flex: none;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(108, 123, 255, 0.12);
  color: var(--text-dim);
  font-size: 12px;
  font-weight: 800;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ov-rk.p1 {
  background: #ffd700;
  color: #4a3800;
}
.ov-rk.p2 {
  background: #c0c8d8;
  color: #333;
}
.ov-rk.p3 {
  background: #cd7f32;
  color: #fff;
}
.ov-nm {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}
.ov-nm b {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
}
.ov-nm small {
  font-size: 10px;
  color: var(--text-dim);
}
.ov-sc {
  color: var(--primary);
  font-weight: 800;
}
.ov-sc em {
  font-style: normal;
  font-size: 11px;
  color: var(--text-dim);
  margin-left: 2px;
}
/* 考试模式中间区：实时逐题作答进度 */
.exam-live {
  padding: 6px 8px;
}
.el-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.el-head h3 {
  font-size: 15px;
  color: var(--text);
}
.el-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
  gap: 10px;
}
.el-cell {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.el-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.el-no {
  font-size: 15px;
  font-weight: 800;
  color: var(--text);
}
.el-rate {
  font-size: 11px;
  color: var(--text-dim);
}
.el-rate.ok {
  color: var(--success);
}
.el-rate.low {
  color: var(--warn);
}
.el-bar {
  height: 5px;
  border-radius: 5px;
  background: rgba(108, 123, 255, 0.12);
  overflow: hidden;
}
.el-fill {
  height: 100%;
  border-radius: 5px;
  background: linear-gradient(90deg, var(--primary), #8a9bff);
  transition: width 0.5s ease;
}
.el-meta {
  font-size: 11px;
  color: var(--text-dim);
}
.el-meta b {
  color: var(--text);
}
.panel {
  min-height: 380px;
}
.panel-title {
  font-size: 14px;
  color: var(--text-dim);
  margin-bottom: 12px;
}
.q-panel {
  padding: 14px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 200px);
  position: sticky;
  top: 14px;
}
.q-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}
.q-count {
  font-size: 12px;
  font-weight: 800;
  color: var(--primary);
  background: rgba(108, 123, 255, 0.12);
  border-radius: 999px;
  padding: 2px 10px;
}
.q-scroll {
  flex: 1;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-right: 4px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  align-content: start;
}
.q-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  border: 1px solid var(--border);
  transition: border-color 0.2s, background 0.2s;
}
.q-item.done {
  border-color: var(--success);
  opacity: 0.55;
}
.q-item.cur {
  background: rgba(108, 123, 255, 0.12);
  border-color: var(--primary);
  opacity: 1;
  box-shadow: 0 0 0 1px var(--primary);
}
.q-item.cur .q-no::after {
  content: '▸';
  margin-left: 4px;
}
.q-mode {
  font-size: 11px;
  font-weight: 800;
  width: 18px;
  height: 18px;
  line-height: 18px;
  text-align: center;
  border-radius: 5px;
  flex: none;
  color: #fff;
  background: var(--primary);
}
.q-item.rush .q-mode {
  background: linear-gradient(135deg, #ff7062, #e0404f);
}
.q-no {
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: var(--primary);
}
.q-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
}
.q-progress {
  flex: 1;
  height: 4px;
  border-radius: 2px;
  background: var(--bg-soft);
  overflow: hidden;
}
.q-progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--primary), var(--primary-strong));
  border-radius: 2px;
  transition: width 0.3s ease;
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
/* 抢答结果条（大屏） */
.rush-result {
  margin-top: 18px;
  display: flex;
  align-items: stretch;
  gap: 12px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background: var(--bg-soft);
  padding: 14px 18px;
}
.rr-hero {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
}
.rr-bolt {
  font-size: 30px;
  line-height: 1;
  filter: drop-shadow(0 0 10px rgba(255, 176, 32, 0.6));
  animation: boltPulse 1.2s ease-in-out infinite;
}
@keyframes boltPulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.15); }
}
.rr-hero-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.rr-hero-text b {
  font-size: 22px;
  font-weight: 800;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.rr-hero-text small {
  font-size: 12px;
  color: var(--warn);
}
.rr-sec {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 0 18px;
  border-left: 1px solid var(--border);
  min-width: 110px;
}
.rr-sec small {
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
}
.rr-ans {
  font-size: 26px;
  font-weight: 800;
  line-height: 1.1;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}
.rr-ans.ok { color: var(--success); }
.rr-ans.no { color: var(--danger); }
@media (prefers-reduced-motion: reduce) {
  .rr-bolt { animation: none; }
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
  .layout,
  .layout.with-right {
    grid-template-columns: 1fr;
  }
  .right-ov {
    position: static;
    max-height: none;
  }
}
</style>
