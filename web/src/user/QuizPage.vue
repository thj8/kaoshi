<template>
  <div class="quiz">
    <!-- 顶栏：标题 + 进度 + 分数 + 倒计时 -->
    <header class="bar">
      <div class="bar-inner">
        <div class="bar-title">
          <b>{{ store.quiz?.title || '…' }}</b>
          <span v-if="store.question" class="bar-q">第 {{ store.question.index }} / {{ store.question.total }} 题</span>
        </div>
        <div class="bar-right">
          <span class="score-chip">{{ store.me?.score ?? 0 }} 分</span>
          <span v-if="remainSec > 0" class="countdown" :class="{ urgent: remainSec <= 5 }">{{ remainSec }}</span>
        </div>
      </div>
      <div v-if="store.question" class="progress">
        <div class="progress-fill" :style="{ width: ((store.question.index - 1) / store.question.total) * 100 + '%' }"></div>
      </div>
    </header>

    <main class="body">
      <!-- 断线提示 -->
      <div v-if="store.wsStatus === 'retrying'" class="offline-tip">连接断开，正在重连…（状态恢复后自动继续）</div>
      <div v-else-if="store.wsStatus === 'connecting'" class="offline-tip dim">连接中…</div>

      <!-- 等待开始 -->
      <section v-if="store.status === 'WAITING'" class="panel hero-panel">
        <div class="big-avatar">{{ (store.me?.nickname || '?').slice(0, 1) }}</div>
        <h1>{{ store.quiz?.title }}</h1>
        <p class="hero-desc">{{ store.quiz?.description }}</p>
        <div class="hero-stats">
          <div><b>{{ store.quiz?.participant_count ?? '—' }}</b><span>参与人数</span></div>
          <div class="vline"></div>
          <div><b>{{ store.me?.nickname || '—' }}</b><span>我的昵称</span></div>
        </div>
        <div class="pulse-dot"></div>
        <p class="wait-text">等待管理员开始答题</p>
      </section>

      <!-- 暂停 -->
      <section v-else-if="store.status === 'PAUSED'" class="panel hero-panel">
        <div class="pause-mark">⏸</div>
        <h1>答题已暂停</h1>
        <p class="hero-desc">请稍候，管理员将继续答题</p>
      </section>

      <!-- 已结束：成绩页 -->
      <section v-else-if="store.status === 'FINISHED'" class="panel hero-panel">
        <p class="finish-eyebrow">答题完成</p>
        <div class="final-score">{{ result?.score ?? store.me?.score ?? 0 }}</div>
        <p class="final-unit">总分 · 排名 #{{ result?.rank ?? '—' }}</p>
        <div class="result-grid">
          <div class="result-item"><div class="num">{{ result?.correct ?? '—' }}</div><div class="lbl">答对</div></div>
          <div class="result-item"><div class="num">{{ result?.wrong ?? '—' }}</div><div class="lbl">答错</div></div>
          <div class="result-item"><div class="num">{{ result ? result.correct_rate.toFixed(0) + '%' : '—' }}</div><div class="lbl">正确率</div></div>
          <div class="result-item"><div class="num">{{ formatDur(result?.duration_sec ?? 0) }}</div><div class="lbl">用时</div></div>
        </div>
        <p class="text-dim finish-total">共 {{ result?.total ?? '—' }} 题</p>

        <div v-if="ranking.length" class="final-ranking">
          <h3>最终排行榜</h3>
          <div v-for="r in ranking.slice(0, 10)" :key="r.user_id" class="rank-row" :class="{ me: r.user_id === store.me?.user_id }">
            <span class="rk" :class="'top' + Math.min(r.rank, 3)">{{ r.rank }}</span>
            <span class="rank-name">{{ r.nickname }}<span v-if="r.user_id === store.me?.user_id" class="text-dim">（我）</span></span>
            <b>{{ r.score }} 分</b>
          </div>
        </div>
        <button class="btn btn-ghost" @click="exit">退出</button>
      </section>

      <!-- 答题中 -->
      <section v-else-if="store.question" class="panel q-panel">
        <div class="q-meta">
          <span class="tag">{{ typeText(store.question.type) }}</span>
          <span class="tag">{{ store.question.score }} 分</span>
          <span class="tag">{{ store.question.required ? '必答' : '可跳过' }}</span>
          <span v-if="isRushQ" class="tag tag-rush">抢答题</span>
        </div>
        <h2 class="q-content">{{ store.question.content }}</h2>

        <!-- 抢答状态提示（按钮固定在页底，见 rush-dock） -->
        <div v-if="rushPhase !== 'idle'" class="rush-panel">
          <div v-if="rushPhase === 'countdown'" class="rush-countdown" :key="rushCd">
            <b :class="{ last: rushCd <= 1 }">{{ rushCd }}</b>
          </div>
          <div v-else class="rush-banner" :class="rushPhase">
            <b>{{ rushBanner[0] }}</b>
            <small>{{ rushBanner[1] }}</small>
          </div>
          <!-- 名额进度（仅抢答窗口内） -->
          <div v-if="store.status === 'RUSHING' && rushPhase !== 'claimed'" class="rush-meter">
            <div class="rush-quota">
              <div class="rush-quota-bar"><div class="rush-quota-fill" :style="{ width: rushQuotaPct }"></div></div>
              <span>已抢 {{ store.rush_winners?.length || 0 }} / {{ rushTotal }}</span>
            </div>
            <span class="rush-remain" :class="{ urgent: remainSec <= 3 && remainSec > 0 }">{{ remainSec }}s</span>
          </div>
        </div>

        <!-- 选项 -->
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
            <span class="opt-badge">{{ o.label }}</span>
            <span class="opt-text">{{ o.content }}</span>
            <span v-if="revealed && reveal?.correct_answer?.includes(o.label)" class="opt-mark ok">✓</span>
            <span v-else-if="revealed && selected.includes(o.label) && !reveal?.correct_answer?.includes(o.label)" class="opt-mark no">✕</span>
          </button>
        </div>

        <!-- 判分反馈 -->
        <div v-if="lastResult" class="feedback" :class="lastResult.is_correct ? 'good' : 'bad'">
          <template v-if="lastResult.is_correct">回答正确 +{{ lastResult.score }} 分 · 总分 {{ lastResult.total_score }}</template>
          <template v-else>回答错误 {{ lastResult.score }} 分 · 总分 {{ lastResult.total_score }}</template>
        </div>

        <!-- 公布答案 -->
        <div v-if="revealed && store.quiz?.show_answer" class="reveal-box">
          <div class="feedback" :class="myReveal.is_correct ? 'good' : 'bad'">
            正确答案：<b>{{ reveal?.correct_answer }}</b>
            <span v-if="myReveal.answered"> · 你的答案：<b :class="myReveal.is_correct ? 't-good' : 't-bad'">{{ myReveal.answer }}</b>（{{ myReveal.score }} 分）</span>
          </div>
          <p v-if="reveal?.analysis && store.quiz?.show_analysis" class="analysis">
            解析：{{ reveal.analysis }}
          </p>
        </div>

        <!-- 时间到（已选答案走 autoSubmit 补交，成功后 submitted=true 显示下方已提交提示） -->
        <div v-if="timeUp && !submitted" class="feedback bad">
          {{ selected.length ? '时间到，自动提交未成功，本题已收卷' : '时间到，本题已收卷，记为未答' }}
        </div>

        <!-- 底部操作 -->
        <div class="actions">
          <div v-if="submitted" class="submitted-tip">已提交 · 等待{{ store.status === 'REVEALING' ? '公布答案' : '下一题' }}…</div>
          <template v-else-if="!timeUp && !optionsLocked">
            <button v-if="!store.question.required" class="btn btn-ghost" @click="skip">跳过本题</button>
            <button class="btn btn-primary submit" :disabled="selected.length === 0" @click="submit">提交答案</button>
          </template>
        </div>
      </section>

      <!-- 排行榜：新标签页打开大屏排行榜 -->
      <div
        v-if="store.quiz?.show_ranking"
        class="rank-fab"
        title="实时排行榜（新窗口）"
        @click="openRank"
      >榜</div>
    </main>

    <!-- 底部固定圆形抢答按钮（页面最重要操作，不随内容滚动） -->
    <div v-if="rushPhase !== 'idle'" class="rush-dock">
      <button class="rush-fab" :class="rushPhase" :disabled="rushPhase !== 'ready'" @click="doRush">
        <template v-if="rushPhase === 'countdown'"><b class="cd" :key="rushCd">{{ rushCd }}</b><small>即将开始</small></template>
        <template v-else-if="rushPhase === 'ready'"><b>抢答</b><small>点击抢答</small></template>
        <template v-else-if="rushPhase === 'claiming'"><b class="cd">…</b><small>提交中</small></template>
        <template v-else-if="rushPhase === 'claimed'"><b class="big-check">✓</b><small>已抢答</small></template>
        <template v-else-if="rushPhase === 'missed'"><b>未抢到</b></template>
        <template v-else-if="rushPhase === 'timeout'"><b>超时</b><small>无人抢答</small></template>
        <template v-else-if="rushPhase === 'ended'"><b>已结束</b><small>等待下一题</small></template>
        <template v-else><b>等待</b></template>
      </button>
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
import { toast } from '../toast'

const route = useRoute()
const router = useRouter()
const store = useQuizStore()
const quizId = String(route.params.id || '')

let ws: QuizWS | null = null
let tickTimer: number | null = null
let qPublishedAt = 0

const selected = ref<string[]>([])
const submitted = ref(false)
const revealed = ref(false)
const timeUp = ref(false)
const reveal = ref<RevealData | null>(null)
// 抢答状态机：waiting/countdown/ready/claiming/claimed/missed/timeout/ended
const rushCd = ref(0) // 3→2→1 开抢倒计时（纯展示层，服务端窗口才是真相）
const lastRushWasRush = ref(false)
let cdTimer: number | null = null
const lastResult = ref<AnswerResultData | null>(null)
const ranking = ref<RankingItem[]>([])
const result = ref<Record<string, any> | null>(null)

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
const rushQuotaPct = computed(() => Math.min(100, ((store.rush_winners?.length || 0) / (rushTotal.value || 1)) * 100) + '%')

/** 抢答单一状态机（避免 boolean 拼接）：idle=非抢答场景 */
type RushPhase = 'idle' | 'waiting' | 'countdown' | 'ready' | 'claiming' | 'claimed' | 'missed' | 'timeout' | 'ended'
const rushPhase = computed<RushPhase>(() => {
  // 终态优先且持久化：抢到/未抢到不因 rush:end 把 status 改成 ANSWERING 而丢失
  // （服务端先广播 rush:end 再单发 rush:success，存在事件顺序竞态）
  if (lastRushWasRush.value && !revealed.value) {
    if (store.rushState === 'won' || store.my_rush_rank > 0) return 'claimed'
    if (store.rushState === 'lost') return 'missed'
  }
  if (store.status === 'RUSHING') {
    if (store.rushState === 'wait') return 'claiming'
    if (rushCd.value > 0) return 'countdown'
    if (store.rushState === 'active') return 'ready'
    return 'waiting'
  }
  if (rushLocked.value) return 'ended'
  if (
    lastRushWasRush.value &&
    store.status === 'ANSWERING' &&
    !revealed.value &&
    (store.rush_winners?.length ?? 0) === 0 &&
    store.my_rush_rank <= 0
  )
    return 'timeout'
  return 'idle'
})
const rushBanner = computed<string[]>(() => {
  switch (rushPhase.value) {
    case 'waiting': return ['等待抢答', '请准备，倒计时结束后开始抢答']
    case 'countdown': return ['抢答即将开始', '倒计时结束后开始抢答']
    case 'ready': return ['请点击下方按钮抢答', `剩余 ${remainSec.value}s · 名额 ${rushTotal.value} 个`]
    case 'claiming': return ['抢答提交中…', '结果以服务端裁定为准']
    case 'claimed': return ['✓ 抢答成功！', '请选择你的答案']
    case 'missed': return ['很遗憾，被抢走了', '请继续参与下一题']
    case 'timeout': return ['抢答超时', '本题无人抢答，请准备下一题']
    case 'ended': return ['本题抢答结束', '请等待下一题']
    default: return []
  }
})

function startRushCountdown() {
  if (cdTimer) { clearInterval(cdTimer); cdTimer = null }
  if (store.my_rush_rank > 0) { rushCd.value = 0; return }
  rushCd.value = 3
  cdTimer = window.setInterval(() => {
    rushCd.value--
    if (rushCd.value <= 0 && cdTimer) { clearInterval(cdTimer); cdTimer = null }
  }, 1000)
}

function syncRemain() {
  if (!store.deadline_at) {
    store.remainMs = 0
    return
  }
  const now = Date.now() + serverOffset
  const r = store.deadline_at - now
  store.remainMs = Math.max(0, r)
  if (r <= 0) onTimeUp()
}

/** 到点统一入口（本地倒计时归零 / 服务端 remain_sec=0 广播）：已选未交自动补交，未选由服务端收卷记未答 */
function onTimeUp() {
  if (submitted.value || timeUp.value) return
  // 暂停/等待/已结束/抢答窗口不在此处理：抢答窗口到期由 rush:end 驱动，暂停时 deadline 冻结（恢复后重算）
  if (store.status !== 'ANSWERING' && store.status !== 'RUNNING') return
  timeUp.value = true
  if (store.question && selected.value.length > 0 && !optionsLocked.value) {
    autoSubmit()
  }
}

/** 到点自动提交：把「已选未交」的答案补交（服务端 SubmitAnswer 允许 deadline+1.5s 宽限） */
async function autoSubmit() {
  const q = store.question
  if (!q || submitted.value) return
  submitted.value = true
  try {
    const r = await userApi.submitAnswer(q.id, selected.value.join(''), Date.now() - qPublishedAt)
    lastResult.value = r
    if (store.me) store.me.score = r.total_score
  } catch {
    // 竞态败给服务端收卷（已记未答）：回退展示，不弹错误打扰用户
    submitted.value = false
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
  if (cdTimer) clearInterval(cdTimer)
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
      if (store.question?.id !== d.question?.id) resetRushState()
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
      if (d.remain_sec === 0) onTimeUp()
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
      store.status = 'RUNNING'
      break
    case Ev.ActivityResume:
      store.status = 'RUNNING'
      // 恢复后服务端按剩余时长重算 deadline，先按 remain_ms 本地校正，
      // 避免旧 deadline 在恢复瞬间误触 onTimeUp
      if (d.remain_ms > 0) {
        store.deadline_at = Date.now() + serverOffset + d.remain_ms
        store.remainMs = d.remain_ms
      }
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
      store.rushState = 'active' // 新窗口重新抢：旧题的 won/lost 不延续（sync 恢复时会纠正）
      store.deadline_at = d.deadline_at || 0
      store.remainMs = d.deadline_at ? Math.max(0, d.deadline_at - Date.now() - serverOffset) : 0
      rushTotal.value = d.winners || 1
      // 新窗口：重置本人抢答状态（除非 sync 已告知成功）
      if (store.my_rush_rank <= 0) {
        store.my_rush_rank = 0
        if (store.rushState !== 'active') store.rushState = 'active'
      }
      resetQuestionUI()
      lastRushWasRush.value = true
      startRushCountdown()
      break

    case Ev.RushSuccess:
      store.rushState = 'won'
      store.rushRank = d.rank
      store.my_rush_rank = d.rank
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
      if (store.rushState !== 'won' && store.rushState !== 'lost') store.rushState = 'ended'
      // 获答者答题倒计时
      store.deadline_at = d.answer_deadline_at || 0
      store.remainMs = d.answer_deadline_at ? Math.max(0, d.answer_deadline_at - Date.now() - serverOffset) : 0
      timeUp.value = false
      if (!store.iAmWinner) submitted.value = true // 非获答者不再提交
      break
  }
}

function resetRushState() {
  store.my_rush_rank = 0
  store.rushRank = 0
  store.rushState = 'idle'
  store.rush_winners = []
  lastRushWasRush.value = false
  rushCd.value = 0
  if (cdTimer) { clearInterval(cdTimer); cdTimer = null }
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

function openRank() {
  window.open(`/rank/${quizId}`, '_blank')
}

async function doRush() {
  if (!store.question || rushPhase.value !== 'ready') return
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
      toast(msg || '抢答失败，请重试')
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
    toast(e?.response?.data?.msg || '提交失败，请重试')
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
.quiz {
  max-width: 680px;
  margin: 0 auto;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* 顶栏 */
.bar {
  position: sticky;
  top: 0;
  z-index: 10;
  background: rgba(255, 255, 255, 0.82);
  backdrop-filter: saturate(1.8) blur(16px);
  -webkit-backdrop-filter: saturate(1.8) blur(16px);
  border-bottom: 1px solid var(--border);
}
.bar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
}
.bar-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
  font-size: 15px;
}
.bar-title b {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.bar-q {
  font-size: 13px;
  color: var(--text-dim);
  white-space: nowrap;
}
.bar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.score-chip {
  font-size: 13px;
  font-weight: 600;
  color: var(--primary);
  background: rgba(0, 113, 227, 0.1);
  padding: 4px 12px;
  border-radius: 999px;
}
.countdown {
  min-width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--primary);
  color: #fff;
  font-size: 17px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.countdown.urgent {
  background: #ff3b30;
  animation: blink 0.6s ease-in-out infinite;
}
@keyframes blink {
  50% { transform: scale(1.08); }
}
.progress {
  height: 3px;
  background: var(--card-2);
}
.progress-fill {
  height: 100%;
  background: var(--primary);
  border-radius: 0 3px 3px 0;
  transition: width 0.4s ease;
}

.body {
  flex: 1;
  padding: 16px 16px 96px;
  width: 100%;
  max-width: 680px;
  margin: 0 auto;
}
/* 底部固定抢答钮出现时，预留空间避免遮挡最后选项 */
.quiz:has(.rush-dock) .body {
  padding-bottom: 190px;
}

/* 断线提示 */
.offline-tip {
  background: rgba(255, 149, 0, 0.12);
  border: 1px solid rgba(255, 149, 0, 0.4);
  color: var(--warn);
  border-radius: 12px;
  padding: 9px 14px;
  margin-bottom: 12px;
  font-size: 13px;
  text-align: center;
}
.offline-tip.dim {
  background: var(--card);
  border-color: var(--border);
  color: var(--text-dim);
}

/* 面板通用 */
.panel {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 24px;
  box-shadow: var(--shadow);
}

/* 居中大屏（等待/暂停/结束） */
.hero-panel {
  text-align: center;
  padding: 48px 24px;
}
.big-avatar {
  width: 84px;
  height: 84px;
  margin: 0 auto 18px;
  border-radius: 50%;
  background: var(--primary);
  color: #fff;
  font-size: 34px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 12px 28px rgba(0, 113, 227, 0.25);
}
.hero-panel h1 {
  font-size: 24px;
  font-weight: 700;
}
.hero-desc {
  color: var(--text-dim);
  font-size: 14px;
  margin-top: 8px;
}
.hero-stats {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 32px;
  margin: 28px 0;
}
.hero-stats b {
  display: block;
  font-size: 22px;
  font-weight: 700;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hero-stats span {
  font-size: 12px;
  color: var(--text-dim);
}
.vline {
  width: 1px;
  height: 36px;
  background: var(--border);
}
.pulse-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--primary);
  margin: 0 auto 12px;
  animation: pulse 1.4s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 0.3; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.15); }
}
.wait-text {
  color: var(--text-dim);
  font-size: 14px;
}
.pause-mark {
  font-size: 40px;
}

/* 成绩页 */
.finish-eyebrow {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.final-score {
  font-size: 72px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.1;
  background: linear-gradient(180deg, #1d1d1f, #6e6e73);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.final-unit {
  color: var(--text-dim);
  font-size: 14px;
  margin-bottom: 24px;
}
.result-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
}
.result-item {
  background: var(--bg);
  border-radius: 14px;
  padding: 14px 6px;
}
.result-item .num {
  font-size: 20px;
  font-weight: 700;
}
.result-item .lbl {
  font-size: 12px;
  color: var(--text-dim);
  margin-top: 2px;
}
.finish-total {
  margin-top: 14px;
  font-size: 13px;
}
.final-ranking {
  margin: 24px 0 20px;
  text-align: left;
}
.final-ranking h3 {
  font-size: 15px;
  margin-bottom: 10px;
}

/* 答题卡片 */
.q-panel {
  padding: 24px 20px;
}
.q-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.tag-rush {
  color: var(--warn);
  background: rgba(255, 149, 0, 0.12);
}
.q-content {
  font-size: 21px;
  font-weight: 650;
  line-height: 1.45;
  letter-spacing: -0.01em;
  margin-bottom: 22px;
}

/* 抢答状态区 + 底部固定圆钮 */
.rush-panel {
  margin-bottom: 20px;
}
.rush-countdown {
  text-align: center;
  padding: 18px 0 6px;
}
.rush-countdown b {
  display: inline-block;
  font-size: 72px;
  font-weight: 800;
  line-height: 1;
  color: var(--primary);
  font-variant-numeric: tabular-nums;
  animation: cdPop 0.3s ease-out;
}
.rush-countdown b.last {
  color: #fa8c16;
}
@keyframes cdPop {
  from { transform: scale(1.35); opacity: 0.2; }
  to { transform: scale(1); opacity: 1; }
}
.rush-banner {
  border-radius: 14px;
  padding: 14px 18px;
  border: 1px solid var(--border);
  background: var(--card);
  display: flex;
  flex-direction: column;
  gap: 3px;
  margin-bottom: 12px;
}
.rush-banner b {
  font-size: 17px;
  font-weight: 700;
  color: var(--text);
}
.rush-banner small {
  font-size: 13px;
  color: var(--text-dim);
}
.rush-banner.claimed {
  border-color: rgba(82, 196, 26, 0.5);
  background: rgba(82, 196, 26, 0.07);
}
.rush-banner.claimed b { color: var(--success); }
.rush-banner.missed, .rush-banner.timeout {
  background: #fafafa;
}
.rush-banner.missed b, .rush-banner.timeout b { color: #8c8c8c; }
.rush-meter {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
  padding: 10px 14px;
  border-radius: 12px;
  background: var(--card);
  border: 1px solid var(--border);
}
.rush-quota {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--text-dim);
}
.rush-quota-bar {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: #f0f1f3;
  overflow: hidden;
}
.rush-quota-fill {
  height: 100%;
  border-radius: 3px;
  background: var(--primary);
  transition: width 0.3s ease;
}
.rush-remain {
  font-size: 18px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  color: var(--text-dim);
  min-width: 44px;
  text-align: right;
}
.rush-remain.urgent {
  color: #fa541c;
  animation: urgentFlash 0.6s steps(2) infinite;
}
@keyframes urgentFlash {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}
/* 底部固定抢答按钮 */
.rush-dock {
  position: fixed;
  bottom: calc(24px + env(safe-area-inset-bottom));
  left: 0;
  right: 0;
  display: flex;
  justify-content: center;
  z-index: 60;
  pointer-events: none;
}
.rush-fab {
  pointer-events: auto;
  width: 136px;
  height: 136px;
  border-radius: 50%;
  border: none;
  font-family: inherit;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  cursor: pointer;
  color: #fff;
  background: var(--card-2);
  box-shadow: 0 6px 22px rgba(0, 0, 0, 0.12);
  transition: transform 0.15s ease, background 0.2s ease, box-shadow 0.2s ease;
}
.rush-fab b {
  font-size: 22px;
  font-weight: 800;
  letter-spacing: 0.02em;
}
.rush-fab b.cd {
  font-size: 40px;
  font-variant-numeric: tabular-nums;
  animation: cdPop 0.3s ease-out;
}
.rush-fab b.big-check { font-size: 44px; }
.rush-fab small {
  font-size: 11px;
  opacity: 0.9;
}
.rush-fab:disabled { cursor: default; }
.rush-fab.ready {
  background: #1677ff;
  box-shadow: 0 8px 28px rgba(22, 119, 255, 0.45);
  animation: fabPulse 1.6s ease-in-out infinite;
}
.rush-fab.ready:active { transform: scale(0.94); }
.rush-fab.countdown { background: #b9c0cc; }
.rush-fab.claiming { background: #5a95e8; }
.rush-fab.claimed {
  background: #52c41a;
  box-shadow: 0 6px 22px rgba(82, 196, 26, 0.35);
}
.rush-fab.missed, .rush-fab.waiting { background: #bfbfbf; color: #fff; }
.rush-fab.timeout { background: rgba(250, 140, 22, 0.75); }
.rush-fab.ended { background: #bfbfbf; }
@keyframes fabPulse {
  0%, 100% { transform: scale(1); box-shadow: 0 8px 28px rgba(22, 119, 255, 0.4); }
  50% { transform: scale(1.03); box-shadow: 0 8px 34px rgba(22, 119, 255, 0.6); }
}
@media (prefers-reduced-motion: reduce) {
  .rush-fab.ready, .rush-remain.urgent, .rush-countdown b, .rush-fab b.cd { animation: none; }
}

/* 选项 */
.opts {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.opts.dimmed {
  opacity: 0.45;
  pointer-events: none;
}
.opt {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 15px 16px;
  border-radius: 16px;
  border: 1.5px solid var(--border);
  background: var(--card);
  color: var(--text);
  font-size: 16px;
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.12s ease, background 0.12s ease, transform 0.08s ease;
  text-align: left;
}
.opt:not(.disabled):active {
  transform: scale(0.985);
}
.opt.sel {
  border-color: var(--primary);
  background: rgba(0, 113, 227, 0.06);
}
.opt.correct {
  border-color: #34c759;
  background: rgba(52, 199, 89, 0.1);
}
.opt.wrong {
  border-color: #ff3b30;
  background: rgba(255, 59, 48, 0.07);
}
.opt.disabled {
  cursor: not-allowed;
}
.opt-badge {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--card-2);
  color: var(--text-dim);
  font-weight: 700;
  flex-shrink: 0;
  transition: background 0.12s ease, color 0.12s ease;
}
.opt.sel .opt-badge {
  background: var(--primary);
  color: #fff;
}
.opt.correct .opt-badge {
  background: #34c759;
  color: #fff;
}
.opt.wrong .opt-badge {
  background: #ff3b30;
  color: #fff;
}
.opt-text {
  flex: 1;
  min-width: 0;
}
.opt-mark {
  font-size: 16px;
  font-weight: 800;
  flex-shrink: 0;
}
.opt-mark.ok { color: var(--success); }
.opt-mark.no { color: var(--danger); }

/* 反馈 */
.feedback {
  border-radius: 14px;
  padding: 13px 16px;
  margin-top: 16px;
  font-size: 15px;
  font-weight: 600;
}
.feedback.good {
  background: rgba(52, 199, 89, 0.12);
  color: var(--success);
}
.feedback.bad {
  background: rgba(255, 59, 48, 0.09);
  color: var(--danger);
}
.analysis {
  font-size: 13px;
  color: var(--text-dim);
  margin-top: 10px;
  line-height: 1.6;
}
.t-good { color: var(--success); }
.t-bad{ color: var(--danger); }

/* 底部操作 */
.actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}
.actions .btn {
  flex: 1;
}
.submitted-tip {
  flex: 1;
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
  padding: 12px;
}

/* 排行榜浮层 */
.rank-fab {
  position: fixed;
  right: 18px;
  bottom: calc(24px + env(safe-area-inset-bottom));
  width: 50px;
  height: 50px;
  border-radius: 50%;
  background: var(--primary);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 8px 24px rgba(0, 113, 227, 0.35);
  z-index: 20;
}
.rank-panel {
  position: fixed;
  right: 18px;
  bottom: calc(86px + env(safe-area-inset-bottom));
  width: 300px;
  max-height: 50vh;
  overflow: auto;
  z-index: 20;
  font-size: 14px;
  padding: 16px;
}
.rank-panel h3 {
  font-size: 15px;
  margin-bottom: 8px;
}
.rank-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 10px;
}
.rank-row.me {
  background: rgba(0, 113, 227, 0.08);
}
.rank-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rank-correct {
  font-size: 12px;
  margin-right: 4px;
}
.rk {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--card-2);
  color: var(--text-dim);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  flex-shrink: 0;
}
.rk.top1 { background: #ffb020; color: #3a2800; }
.rk.top2 { background: #c7cbd4; color: #2a2d36; }
.rk.top3 { background: #d18b4d; color: #38190a; }

@media (max-width: 640px) {
  .result-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .q-content {
    font-size: 19px;
  }
  .opt {
    font-size: 15px;
    padding: 14px;
  }
  .final-score {
    font-size: 60px;
  }
}
</style>
