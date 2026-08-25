<template>
  <div class="page quiz-page">
    <!-- 顶栏 -->
    <div class="card topbar">
      <div class="topbar-title">
        <b>{{ store.quiz?.title || '...' }}</b>
        <span v-if="store.question" class="text-dim" style="font-size: 13px">
          第 {{ store.question.index }} / {{ store.question.total }} 题
        </span>
      </div>
      <div class="topbar-right">
        <span class="score-chip">🏅 {{ store.me?.score ?? 0 }} 分</span>
        <span v-if="remainSec > 0" class="countdown" :class="{ urgent: remainSec <= 5 }">{{ remainSec }}</span>
      </div>
    </div>

    <!-- 断线提示 -->
    <div v-if="store.wsStatus === 'retrying'" class="offline-tip">⚠️ 连接断开，正在重连...（状态恢复后自动继续）</div>
    <div v-else-if="store.wsStatus === 'connecting'" class="offline-tip dim">连接中...</div>

    <!-- 等待开始 -->
    <div v-if="store.status === 'WAITING'" class="card center-card">
      <h1 style="font-size: 22px; margin-bottom: 8px">{{ store.quiz?.title }}</h1>
      <p class="text-dim" style="margin-bottom: 24px">{{ store.quiz?.description }}</p>
      <div style="display: flex; gap: 28px; margin-bottom: 28px">
        <div><div style="font-size: 24px; font-weight: 800">{{ store.quiz?.participant_count ?? '—' }}</div><div class="text-dim" style="font-size: 12px">参与人数</div></div>
        <div><div style="font-size: 24px; font-weight: 800">{{ store.me?.nickname || '—' }}</div><div class="text-dim" style="font-size: 12px">我的昵称</div></div>
      </div>
      <div class="pulse-dot"></div>
      <p style="margin-top: 12px; color: var(--primary)">等待管理员开始答题...</p>
    </div>

    <!-- 暂停 -->
    <div v-else-if="store.status === 'PAUSED'" class="card center-card">
      <div style="font-size: 40px">⏸️</div>
      <p style="margin-top: 12px">答题已暂停，请等待继续</p>
    </div>

    <!-- 已结束：成绩页 -->
    <div v-else-if="store.status === 'FINISHED'" class="card center-card">
      <div style="font-size: 44px">🎉</div>
      <h2 style="margin: 10px 0 4px">答题完成</h2>
      <div class="result-grid">
        <div class="result-item"><div class="num">{{ result?.score ?? store.me?.score ?? 0 }}</div><div class="lbl">总分</div></div>
        <div class="result-item"><div class="num">{{ result?.correct ?? '—' }}</div><div class="lbl">答对</div></div>
        <div class="result-item"><div class="num">{{ result?.wrong ?? '—' }}</div><div class="lbl">答错</div></div>
        <div class="result-item"><div class="num">{{ result ? result.correct_rate.toFixed(0) + '%' : '—' }}</div><div class="lbl">正确率</div></div>
        <div class="result-item"><div class="num">#{{ result?.rank ?? '—' }}</div><div class="lbl">排名</div></div>
      </div>
      <p class="text-dim" style="margin-top: 16px; font-size: 13px">
        共 {{ result?.total ?? '—' }} 题 · 用时 {{ formatDur(result?.duration_sec ?? 0) }}
      </p>

      <div v-if="ranking.length" style="margin-top: 20px; text-align: left; width: 100%">
        <h3 style="margin-bottom: 10px">🏆 最终排行榜</h3>
        <div v-for="r in ranking.slice(0, 10)" :key="r.user_id" class="rank-row" :class="{ me: r.user_id === store.me?.user_id }">
          <span class="rk" :class="'top' + Math.min(r.rank, 3)">{{ r.rank }}</span>
          <span style="flex: 1">{{ r.nickname }}<span v-if="r.user_id === store.me?.user_id" class="text-dim">（我）</span></span>
          <b>{{ r.score }} 分</b>
        </div>
      </div>
      <button class="btn btn-ghost" style="margin-top: 20px" @click="exit">退出</button>
    </div>

    <!-- 答题中 -->
    <div v-else-if="store.question" class="card q-card">
      <div class="q-meta">
        <span class="tag">{{ typeText(store.question.type) }}</span>
        <span class="tag">{{ store.question.score }} 分</span>
        <span class="tag">{{ store.question.required ? '必答' : '可跳过' }}</span>
        <span v-if="isRushQ" class="tag" style="color: var(--warn)">⚡ 抢答题</span>
      </div>
      <h2 class="q-content">{{ store.question.content }}</h2>

      <!-- 抢答面板（抢答窗口进行中 / 本题为抢答题且我未获答） -->
      <div v-if="store.status === 'RUSHING' || rushLocked" class="rush-panel">
        <!-- 窗口进行中：大抢答按钮四态 -->
        <template v-if="store.status === 'RUSHING'">
          <button v-if="store.rushState === 'active'" class="rush-btn active" @click="doRush">
            🔥<span>立即抢答</span>
          </button>
          <div v-else-if="store.rushState === 'won'" class="rush-state won">
            🎉 抢答成功！<small>第 {{ store.rushRank }} 名 · 你获得本题答题资格（+{{ lastRushBonus }}分）</small>
          </div>
          <div v-else-if="store.rushState === 'lost'" class="rush-state lost">
            很遗憾<small>本题抢答资格已被其他用户获得</small>
          </div>
          <div v-else class="rush-state wait">等待抢答...<small>手速决定胜负</small></div>
          <p class="text-dim" style="margin-top: 10px; font-size: 12px">
            已抢 {{ store.rush_winners?.length || 0 }} / {{ rushTotal }} · 剩余 {{ remainSec }}s
          </p>
        </template>
        <!-- 窗口已结束但我是非获答者 -->
        <div v-else-if="rushLocked" class="rush-state ended">
          本题抢答结束<small>获答者：<template v-for="(w, i) in store.rush_winners" :key="w.user_id"><template v-if="i">、</template>{{ w.nickname }}</template></small>
        </div>
      </div>

      <!-- 选项（非获答者在抢答题中不可操作） -->
      <div class="opts" :class="{ dimmed: optionsLocked }">
        <button
          v-for="o in store.question.options"
          :key="o.label"
          class="opt"
          :class="{
            sel: selected.includes(o.label),
            correct: revealed && reveal?.correct_answer?.includes(o.label),
            wrong: revealed && selected.includes(o.label) && !reveal?.correct_answer?.includes(o.label),
            disabled: submitted || timeUp || optionsLocked,
          }"
          @click="toggle(o.label)"
        >
          <span class="opt-label">{{ o.label }}</span>
          <span style="flex: 1; text-align: left">{{ o.content }}</span>
        </button>
      </div>

      <!-- 判分反馈 -->
      <div v-if="lastResult" class="feedback" :class="lastResult.is_correct ? 'good' : 'bad'">
        <template v-if="lastResult.is_correct">✅ 回答正确！ +{{ lastResult.score }} 分（总分 {{ lastResult.total_score }}）</template>
        <template v-else>❌ 回答错误 {{ lastResult.score }} 分（总分 {{ lastResult.total_score }}）</template>
      </div>

      <!-- 公布答案反馈 -->
      <div v-if="revealed && store.quiz?.show_answer" class="reveal-box">
        <div class="feedback" :class="myReveal.is_correct ? 'good' : 'bad'">
          正确答案：<b>{{ reveal?.correct_answer }}</b>
          <span v-if="myReveal.answered"> · 你的答案：<b :class="myReveal.is_correct ? 't-good' : 't-bad'">{{ myReveal.answer }}</b>（{{ myReveal.score }} 分）</span>
        </div>
        <p v-if="reveal?.analysis && store.quiz?.show_analysis" class="text-dim" style="font-size: 13px; margin-top: 8px">
          💡 {{ reveal.analysis }}
        </p>
      </div>

      <!-- 时间到 -->
      <div v-if="timeUp && !submitted" class="feedback bad">⏰ 时间到，本题已自动提交</div>

      <!-- 底部操作 -->
      <div class="actions">
        <button v-if="!store.question.required && !submitted && !optionsLocked" class="btn btn-ghost" style="flex: 1" @click="skip">跳过本题</button>
        <button
          v-if="!submitted && !timeUp && !optionsLocked"
          class="btn btn-primary"
          style="flex: 2"
          :disabled="selected.length === 0"
          @click="submit"
        >
          提交答案
        </button>
        <div v-if="submitted" class="text-dim" style="flex: 2; text-align: center; padding: 12px">
          已提交 · 等待{{ store.status === 'REVEALING' ? '公布答案' : '下一题' }}...
        </div>
      </div>
    </div>

    <!-- 排行榜浮动按钮（进行中） -->
    <div v-if="store.quiz?.show_ranking && store.status !== 'WAITING' && store.status !== 'FINISHED'" class="rank-fab" @click="showRanking = !showRanking">
      🏆
    </div>
    <div v-if="showRanking" class="card rank-panel">
      <h3 style="margin-bottom: 10px">🏆 实时排行榜</h3>
      <div v-for="r in ranking" :key="r.user_id" class="rank-row" :class="{ me: r.user_id === store.me?.user_id }">
        <span class="rk" :class="'top' + Math.min(r.rank, 3)">{{ r.rank }}</span>
        <span style="flex: 1">{{ r.nickname }}</span>
        <span class="text-dim" style="font-size: 12px; margin-right: 10px">对{{ r.correct }}</span>
        <b>{{ r.score }} 分</b>
      </div>
      <p v-if="ranking.length === 0" class="text-dim">暂无数据</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { QuizWS } from '../ws'
import { Ev, type RankingItem, type RevealData, type AnswerResultData, type WSMessage } from '../ws/types'
import { useQuizStore } from '../stores/quiz'
import { userApi } from '../api/user'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()
const store = useQuizStore()
const quizId = Number(route.params.id)

let ws: QuizWS | null = null
let tickTimer: number | null = null
let qPublishedAt = 0

const selected = ref<string[]>([])
const submitted = ref(false)
const revealed = ref(false)
const timeUp = ref(false)
const reveal = ref<RevealData | null>(null)
const lastResult = ref<AnswerResultData | null>(null)
const ranking = ref<RankingItem[]>([])
const result = ref<Record<string, any> | null>(null)
const showRanking = ref(false)
const lastRushBonus = ref(0)
const rushTotal = ref(1)

/** 服务器时间偏移（server_now - Date.now()） */
let serverOffset = 0

const remainSec = computed(() => Math.ceil(store.remainMs / 1000))

/** 公布答案时的个人反馈：优先服务端 reveal 携带的数据（刷新/未提交也有），回退本地 lastResult */
const myReveal = computed(() => {
  const rv = reveal.value
  if (rv && rv.my_answer !== undefined) {
    return { answered: rv.my_answer !== '-', answer: rv.my_answer, score: rv.my_score ?? 0, is_correct: !!rv.is_correct }
  }
  const lr = lastResult.value
  if (lr) return { answered: true, answer: lr.answer, score: lr.score, is_correct: lr.is_correct }
  return { answered: false, answer: '', score: 0, is_correct: false }
})

/** 抢答窗口已结束且我不是获答者：锁定作答 */
const rushLocked = computed(
  () => store.status !== 'RUSHING' && (store.rush_winners?.length ?? 0) > 0 && store.my_rush_rank <= 0 && !revealed.value
)
/** 选项是否可操作（抢答未获答 / 窗口进行中且我未抢中） */
const optionsLocked = computed(
  () => rushLocked.value || (store.status === 'RUSHING' && !store.iAmWinner)
)
/** 是否为抢答题（窗口进行中，或已有获答名单） */
const isRushQ = computed(() => store.status === 'RUSHING' || (store.rush_winners?.length ?? 0) > 0)

function syncRemain() {
  if (!store.deadline_at) {
    store.remainMs = 0
    return
  }
  const now = Date.now() + serverOffset
  const r = store.deadline_at - now
  store.remainMs = Math.max(0, r)
  if (r <= 0 && !submitted.value && !timeUp.value) {
    timeUp.value = true
  }
}

onMounted(async () => {
  const token = userApi.quizToken(quizId)
  if (!token) {
    router.replace('/join')
    return
  }

  ws = new QuizWS({
    token,
    onStatus: (s) => (store.wsStatus = s),
    onEvent: (msg: WSMessage) => handleEvent(msg),
  })

  // 本地倒计时渲染（展示层）
  tickTimer = window.setInterval(syncRemain, 200)
})

onUnmounted(() => {
  ws?.close()
  if (tickTimer) clearInterval(tickTimer)
})

function handleEvent(msg: WSMessage) {
  const d = msg.data || {}
  switch (msg.event) {
    case Ev.Sync:
      serverOffset = (d.server_now || Date.now()) - Date.now()
      store.applySync(d)
      resetQuestionUI()
      if (d.status === 'FINISHED') loadResult()
      break

    case Ev.QuestionPublish:
      serverOffset = (d.server_now || Date.now()) - Date.now()
      store.question = d.question
      store.deadline_at = d.deadline_at || 0
      store.status = d.status || 'ANSWERING'
      store.remainMs = d.deadline_at ? Math.max(0, d.deadline_at - Date.now() - serverOffset) : 0
      resetQuestionUI()
      qPublishedAt = Date.now()
      break

    case Ev.QuestionCountdown:
      if (d.deadline_at) {
        store.deadline_at = d.deadline_at
        store.remainMs = Math.max(0, d.deadline_at - Date.now() - serverOffset)
      } else if (d.remain_sec !== undefined) {
        store.remainMs = d.remain_sec * 1000
      }
      if (d.remain_sec === 0 && !submitted.value) timeUp.value = true
      break

    case Ev.AnswerResult:
      // 即时个人结果（无正确答案）
      if (!d.revealed) {
        lastResult.value = d
        submitted.value = true
        if (store.me) store.me.score = d.total_score
      }
      break

    case Ev.AnswerReveal:
      revealed.value = true
      reveal.value = d
      if (store.quiz) store.status = 'REVEALING'
      // 补充个人对错（若提交过）
      if (lastResult.value) lastResult.value.revealed = true
      break

    case Ev.ActivityStart:
    case Ev.ActivityResume:
      store.status = 'RUNNING'
      break
    case Ev.ActivityPause:
      store.status = 'PAUSED'
      break
    case Ev.ActivityEnd:
      store.status = 'FINISHED'
      if (d.ranking) ranking.value = d.ranking
      loadResult()
      break

    case Ev.RankingUpdate:
      ranking.value = d.items || []
      break

    case Ev.RushStart:
      store.status = 'RUSHING'
      store.rush_active = true
      store.rushState = store.rushState === 'idle' || store.rushState === 'ended' ? 'active' : store.rushState
      store.deadline_at = d.deadline_at || 0
      store.remainMs = d.deadline_at ? Math.max(0, d.deadline_at - Date.now() - serverOffset) : 0
      rushTotal.value = d.winners || 1
      // 新窗口：重置本人抢答状态（除非 sync 已告知成功）
      if (store.my_rush_rank <= 0) {
        store.my_rush_rank = 0
        if (store.rushState !== 'active') store.rushState = 'active'
      }
      resetQuestionUI()
      break

    case Ev.RushSuccess:
      store.rushState = 'won'
      store.rushRank = d.rank
      store.my_rush_rank = d.rank
      lastRushBonus.value = d.bonus
      if (store.me) store.me.score = d.score
      break

    case Ev.RushFailed:
      store.rushState = 'lost'
      store.my_rush_rank = -1
      break

    case Ev.RushEnd:
      store.status = 'ANSWERING'
      store.rush_active = false
      store.rush_winners = d.winners || []
      if (store.rushState !== 'won') store.rushState = 'ended'
      // 获答者答题倒计时
      store.deadline_at = d.answer_deadline_at || 0
      store.remainMs = d.answer_deadline_at ? Math.max(0, d.answer_deadline_at - Date.now() - serverOffset) : 0
      timeUp.value = false
      if (!store.iAmWinner) submitted.value = true // 非获答者不再提交
      break
  }
}

function resetQuestionUI() {
  selected.value = []
  submitted.value = false
  revealed.value = false
  timeUp.value = false
  reveal.value = null
  lastResult.value = null
  // 抢答状态由 sync/rush 事件单独维护，这里不重置 my_rush_rank
}

function toggle(label: string) {
  if (submitted.value || timeUp.value) return
  const q = store.question
  if (!q) return
  if (q.type === 'single' || q.type === 'judge') {
    selected.value = [label]
  } else {
    const i = selected.value.indexOf(label)
    if (i >= 0) selected.value.splice(i, 1)
    else selected.value.push(label)
    selected.value.sort()
  }
}

async function doRush() {
  if (!store.question || store.rushState !== 'active') return
  store.rushState = 'wait' // 防连点，等待服务器裁决
  try {
    await userApi.rush(store.question.id)
    // 结果由 WS rush:success / rush:failed 事件驱动 UI
  } catch (e: any) {
    const msg = e?.response?.data?.msg || ''
    if (msg.includes('已抢答')) {
      // 幂等：已有记录，等 WS 事件或恢复为 won/lost
    } else if (msg.includes('很遗憾') || msg.includes('资格已被')) {
      store.rushState = 'lost'
      store.my_rush_rank = -1
    } else {
      store.rushState = 'active' // 可重试（如网络错误）
      alert(msg || '抢答失败，请重试')
    }
  }
}

async function submit() {
  if (!store.question || selected.value.length === 0) return
  submitted.value = true
  try {
    const r = await userApi.submitAnswer(store.question.id, selected.value.join(''), Date.now() - qPublishedAt)
    lastResult.value = r
    if (store.me) store.me.score = r.total_score
  } catch (e: any) {
    submitted.value = false
    alert(e?.response?.data?.msg || '提交失败，请重试')
  }
}

async function skip() {
  // 非必答题跳过：本地标记，等服务端发布下一题
  submitted.value = true
  lastResult.value = null
}

async function loadResult() {
  try {
    result.value = await userApi.result(quizId)
    const rk = await userApi.ranking(quizId)
    if (rk.visible && rk.items) ranking.value = rk.items
  } catch {
    /* ignore */
  }
}

function exit() {
  localStorage.removeItem(LS.userToken(quizId))
  router.replace('/join')
}

function typeText(t: string) {
  return ({ single: '单选', multiple: '多选', judge: '判断' } as Record<string, string>)[t] || t
}

function formatDur(sec: number) {
  const m = Math.floor(sec / 60)
  const s = sec % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}
</script>

<style scoped>
.quiz-page {
  max-width: 720px;
}
.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 18px;
  margin-bottom: 12px;
  position: sticky;
  top: 8px;
  z-index: 10;
  backdrop-filter: blur(6px);
}
.topbar-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.score-chip {
  background: var(--card-2);
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 14px;
}
.countdown {
  min-width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  background: linear-gradient(135deg, var(--primary), var(--primary-strong));
  font-size: 20px;
  font-weight: 800;
}
.countdown.urgent {
  background: linear-gradient(135deg, #ff7062, #e0404f);
  animation: blink 0.6s ease-in-out infinite;
}
@keyframes blink {
  50% { transform: scale(1.08); }
}
.offline-tip {
  background: rgba(255, 176, 32, 0.12);
  border: 1px solid var(--warn);
  color: var(--warn);
  border-radius: 10px;
  padding: 8px 14px;
  margin-bottom: 12px;
  font-size: 13px;
  text-align: center;
}
.offline-tip.dim {
  background: var(--card);
  border-color: var(--border);
  color: var(--text-dim);
}
.center-card {
  text-align: center;
  padding: 48px 24px;
}
.pulse-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--primary);
  margin: 0 auto;
  animation: pulse 1.4s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 0.35; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.15); }
}
.result-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-top: 20px;
}
.result-item .num {
  font-size: 22px;
  font-weight: 800;
}
.result-item .lbl {
  font-size: 12px;
  color: var(--text-dim);
}
.q-card {
  padding: 22px;
}
/* 抢答面板 */
.rush-panel {
  margin-bottom: 18px;
}
.rush-btn {
  width: 100%;
  padding: 34px 20px;
  border: none;
  border-radius: 18px;
  background: linear-gradient(135deg, #ff7062, #e0404f);
  color: #fff;
  font-size: 26px;
  font-weight: 900;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  box-shadow: 0 8px 28px rgba(224, 64, 79, 0.4);
  animation: rushPulse 1.2s ease-in-out infinite;
}
.rush-btn:active {
  transform: scale(0.96);
}
.rush-btn small {
  font-size: 13px;
  font-weight: 500;
  opacity: 0.9;
}
@keyframes rushPulse {
  0%, 100% { box-shadow: 0 8px 28px rgba(224, 64, 79, 0.35); }
  50% { box-shadow: 0 8px 36px rgba(224, 64, 79, 0.65); transform: scale(1.015); }
}
.rush-state {
  border-radius: 16px;
  padding: 26px 20px;
  text-align: center;
  font-size: 19px;
  font-weight: 800;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.rush-state small {
  font-size: 13px;
  font-weight: 500;
  opacity: 0.85;
}
.rush-state.won {
  background: rgba(46, 204, 143, 0.14);
  color: var(--success);
  border: 1px solid var(--success);
}
.rush-state.lost {
  background: rgba(255, 93, 108, 0.12);
  color: var(--danger);
  border: 1px solid var(--danger);
}
.rush-state.wait {
  background: var(--bg-soft);
  color: var(--text-dim);
  border: 1px dashed var(--border);
}
.rush-state.ended {
  background: var(--bg-soft);
  color: var(--text-dim);
  border: 1px solid var(--border);
}
.opts.dimmed {
  opacity: 0.45;
  pointer-events: none;
}
.q-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
.q-content {
  font-size: 19px;
  line-height: 1.5;
  margin-bottom: 20px;
}
.opts {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.opt {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 15px 18px;
  border-radius: 14px;
  border: 2px solid var(--border);
  background: var(--bg-soft);
  color: var(--text);
  font-size: 16px;
  cursor: pointer;
  transition: all 0.12s ease;
  text-align: left;
}
.opt:not(.disabled):active {
  transform: scale(0.98);
}
.opt.sel {
  border-color: var(--primary);
  background: rgba(108, 123, 255, 0.14);
}
.opt.correct {
  border-color: var(--success);
  background: rgba(46, 204, 143, 0.14);
}
.opt.wrong {
  border-color: var(--danger);
  background: rgba(255, 93, 108, 0.14);
}
.opt.disabled {
  cursor: not-allowed;
}
.opt-label {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--card-2);
  font-weight: 800;
  flex-shrink: 0;
}
.opt.sel .opt-label {
  background: var(--primary);
}
.feedback {
  border-radius: 12px;
  padding: 13px 16px;
  margin-top: 16px;
  font-size: 15px;
  font-weight: 600;
}
.feedback.good {
  background: rgba(46, 204, 143, 0.14);
  color: var(--success);
}
.feedback.bad {
  background: rgba(255, 93, 108, 0.12);
  color: var(--danger);
}
.reveal-box {
  margin-top: 4px;
}
.t-good { color: var(--success); }
.t-bad { color: var(--danger); }
.actions {
  display: flex;
  gap: 12px;
  margin-top: 22px;
}
.rank-fab {
  position: fixed;
  right: 18px;
  bottom: 24px;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--primary), var(--primary-strong));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  cursor: pointer;
  box-shadow: 0 6px 20px rgba(108, 123, 255, 0.4);
  z-index: 20;
}
.rank-panel {
  position: fixed;
  right: 18px;
  bottom: 88px;
  width: 300px;
  max-height: 50vh;
  overflow: auto;
  z-index: 20;
  font-size: 14px;
}
.rank-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px;
  border-radius: 8px;
}
.rank-row.me {
  background: rgba(108, 123, 255, 0.15);
}
.rk {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: var(--card-2);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 800;
}
.rk.top1 { background: #ffb020; color: #1a1200; }
.rk.top2 { background: #b8c0d8; color: #10142a; }
.rk.top3 { background: #d18b4d; color: #1a0e00; }

@media (max-width: 640px) {
  .result-grid {
    grid-template-columns: repeat(3, 1fr);
  }
  .opt {
    padding: 14px;
    font-size: 15px;
  }
}
</style>
