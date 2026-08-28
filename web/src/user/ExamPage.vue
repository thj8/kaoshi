<template>
  <div class="exam">
    <!-- 顶栏：考试名 + 进度 + 剩余时间 -->
    <header class="bar">
      <div class="bar-inner">
        <div class="bar-title">
          <b>{{ paper?.title || '…' }}</b>
          <span v-if="paper" class="bar-q">{{ status === 'WAITING' ? `共 ${displayTotal} 道题` : `第 ${curIndex + 1} / ${paper.total} 题` }}</span>
        </div>
        <div class="bar-right">
          <span class="answered-chip">已答 {{ answeredCount }} / {{ displayTotal || '—' }}</span>
          <span
            v-if="remainText"
            class="countdown"
            :class="{ urgent: remainMs <= 5 * 60 * 1000 && remainMs > 0 }"
          >{{ remainText }}</span>
        </div>
      </div>
      <div v-if="paper && displayTotal > 0" class="progress">
        <div class="progress-fill" :style="{ width: (answeredCount / displayTotal) * 100 + '%' }"></div>
      </div>
    </header>

    <main class="body">
      <!-- 断线提示 -->
      <div v-if="wsStatus === 'retrying'" class="offline-tip">连接断开，正在重连…（状态恢复后自动继续）</div>
      <div v-else-if="wsStatus === 'connecting'" class="offline-tip dim">连接中…</div>

      <!-- 等待开考 -->
      <section v-if="status === 'WAITING'" class="panel hero-panel">
        <div class="big-avatar">{{ nick.slice(0, 1) }}</div>
        <h1>{{ paper?.title || '考试' }}</h1>
        <p class="hero-desc">共 {{ displayTotal }} 道题 · 自由切题 · 交卷前可随时修改答案</p>
        <div class="pulse-dot"></div>
        <p class="wait-text">等待管理员开始考试</p>
      </section>

      <!-- 考试中 -->
      <template v-else-if="examVisible && curQ">
        <!-- 移动端索引折叠条 -->
        <button class="idx-toggle" @click="indexOpen = !indexOpen">
          <span>题目索引 · 已答 {{ answeredCount }}/{{ paper!.total }} · 标记 {{ markedCount }}</span>
          <i>{{ indexOpen ? '收起 ▲' : '展开 ▼' }}</i>
        </button>

        <div class="cols">
          <!-- 左：题目卡 -->
          <section class="panel q-panel">
            <div class="q-meta">
              <span class="tag">{{ typeText(curQ.type) }}</span>
              <span class="tag">{{ curQ.score }} 分</span>
              <button class="mark-btn" :class="{ on: marks[curQ.id] }" @click="toggleMark(curQ.id)">
                {{ marks[curQ.id] ? '★ 已标记' : '☆ 标记本题' }}
              </button>
            </div>
            <h2 class="q-content">{{ curQ.content }}</h2>

            <!-- 选项：选择即自动保存 -->
            <div class="opts">
              <button
                v-for="o in curQ.options"
                :key="o.label"
                class="opt"
                :class="{ sel: answers[curQ.id]?.includes(o.label) }"
                @click="toggle(o.label)"
              >
                <span class="opt-badge">{{ o.label }}</span>
                <span class="opt-text">{{ o.content }}</span>
              </button>
            </div>

            <p class="save-hint" :class="{ show: saveState !== '' }">
              {{ saveState === 'saving' ? '保存中…' : '已自动保存 ✓' }}
            </p>

            <!-- 底部导航：上一题 / 题号 / 下一题 -->
            <div class="nav">
              <button class="btn btn-ghost nav-btn" :disabled="curIndex <= 0" @click="go(curIndex - 1)">← 上一题</button>
              <span class="nav-num">{{ curIndex + 1 }} <i>/</i> {{ paper!.total }}</span>
              <button class="btn btn-ghost nav-btn" :disabled="curIndex >= paper!.total - 1" @click="go(curIndex + 1)">下一题 →</button>
            </div>
          </section>

          <!-- 右：题目索引卡（桌面端 sticky 固定） -->
          <aside class="panel idx-panel" :class="{ open: indexOpen }">
            <h3 class="idx-title">题目索引</h3>
            <div class="qix">
              <button
                v-for="(q, i) in paper!.questions"
                :key="q.id"
                class="qix-box"
                :class="{ done: (answers[q.id]?.length ?? 0) > 0, mark: marks[q.id], cur: i === curIndex }"
                @click="go(i)"
              >{{ i + 1 }}</button>
            </div>
            <div class="legend">
              <span><i class="lg lg-todo"></i>未答</span>
              <span><i class="lg lg-done"></i>已答</span>
              <span><i class="lg lg-cur"></i>当前题</span>
              <span><i class="lg lg-mark"></i>已标记</span>
            </div>
            <button class="btn submit-all" @click="askSubmit">交卷</button>
          </aside>
        </div>
      </template>

      <!-- 成绩 / 已交卷 -->
      <section v-else class="panel hero-panel">
        <template v-if="result">
          <p class="finish-eyebrow">{{ status === 'FINISHED' ? '考试结束' : '已交卷' }}</p>
          <div class="final-score">{{ result.score }}</div>
          <p class="final-unit">
            总分 · 排名 #{{ result.rank ?? '—' }}
            <span v-if="status !== 'FINISHED'" class="text-dim">（等待考试结束后公布完整排行）</span>
          </p>
          <div class="result-grid">
            <div class="result-item"><div class="num">{{ result.correct ?? '—' }}</div><div class="lbl">答对</div></div>
            <div class="result-item"><div class="num">{{ result.wrong ?? '—' }}</div><div class="lbl">答错</div></div>
            <div class="result-item"><div class="num">{{ result ? Number(result.correct_rate).toFixed(0) + '%' : '—' }}</div><div class="lbl">正确率</div></div>
            <div class="result-item"><div class="num">{{ formatDur(result.duration_sec ?? 0) }}</div><div class="lbl">用时</div></div>
          </div>
          <p class="text-dim finish-total">共 {{ result.total ?? '—' }} 题 · 已答 {{ result.answered ?? 0 }} 题</p>

          <div v-if="status === 'FINISHED' && finalRanking.length" class="final-ranking">
            <h3>最终排行榜</h3>
            <div
              v-for="r in finalRanking.slice(0, 10)"
              :key="r.user_id"
              class="rank-row"
              :class="{ me: r.user_id === meId }"
            >
              <span class="rk" :class="'top' + Math.min(r.rank, 3)">{{ r.rank }}</span>
              <span class="rank-name">{{ r.nickname }}<span v-if="r.user_id === meId" class="text-dim">（我）</span></span>
              <b>{{ r.score }} 分</b>
            </div>
          </div>
        </template>
        <template v-else>
          <div class="spinner"></div>
          <p class="wait-text">{{ submitted ? '成绩计算中…' : '考试时间到，正在收卷…' }}</p>
        </template>
        <button class="btn btn-ghost exit-btn" @click="exit">退出</button>
      </section>
    </main>

    <!-- 交卷确认弹窗 -->
    <div v-if="showSubmitModal" class="modal-mask" @click.self="showSubmitModal = false">
      <div class="modal panel">
        <h3>确认交卷？</h3>
        <p class="modal-msg">
          <template v-if="unanswered > 0">
            还有 <b class="t-bad">{{ unanswered }}</b> 道题未作答，交卷后将无法继续答题。
          </template>
          <template v-else>所有题目均已完成，确认现在提交试卷吗？</template>
        </p>
        <div class="modal-actions">
          <button class="btn btn-ghost" @click="showSubmitModal = false">继续答题</button>
          <button class="btn btn-danger" :disabled="submitting" @click="doSubmit">
            {{ submitting ? '提交中…' : '确认交卷' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { QuizWS } from '../ws'
import { Ev, type RankingItem, type WSMessage } from '../ws/types'
import { userApi, type Paper, type PaperQuestion } from '../api/user'
import { LS } from '../api'
import { toast } from '../toast'

const route = useRoute()
const router = useRouter()
const quizId = String(route.params.id || '')

let ws: QuizWS | null = null
let tickTimer: number | null = null
const saveTimers = new Map<number, number>()

const nick = ref('我')
const meId = ref(0)
const wsStatus = ref('closed')
const status = ref('WAITING')
const paper = ref<Paper | null>(null)
const curIndex = ref(0)
const answers = ref<Record<number, string[]>>({})
const marks = ref<Record<number, boolean>>({})
const indexOpen = ref(false)

const submitted = ref(false)
const showSubmitModal = ref(false)
const submitting = ref(false)
const saveState = ref('') // '' | 'saving' | 'saved'

const result = ref<Record<string, any> | null>(null)
const finalRanking = ref<RankingItem[]>([])
let resultLoaded = false

/** 服务器时间偏移（server_now - Date.now()） */
let serverOffset = 0
const deadlineAt = ref(0)
const remainMs = ref(0)

const curQ = computed<PaperQuestion | null>(() => paper.value?.questions[curIndex.value] ?? null)
// 等待页展示真实题数（WAITING 时 total=0 防提前看题，仅下发 question_count）
const displayTotal = computed(() =>
  status.value === 'WAITING' ? paper.value?.question_count ?? 0 : paper.value?.total ?? 0
)
const examVisible = computed(
  () => !!paper.value && !submitted.value && status.value === 'RUNNING' && !(deadlineAt.value > 0 && remainMs.value <= 0)
)
const answeredCount = computed(
  () => paper.value?.questions.filter((q) => (answers.value[q.id]?.length ?? 0) > 0).length ?? 0
)
const unanswered = computed(() => (paper.value?.total ?? 0) - answeredCount.value)
const markedCount = computed(() => Object.values(marks.value).filter(Boolean).length)
const remainText = computed(() => {
  if (deadlineAt.value <= 0) return ''
  return formatHMS(Math.ceil(remainMs.value / 1000))
})

// ---------- 标记（本地持久化，橙色状态） ----------
const marksKey = `kaoshi_exam_marks_${quizId}_${localStorage.getItem(LS.userId(quizId)) || ''}`
function loadMarks() {
  try {
    marks.value = JSON.parse(localStorage.getItem(marksKey) || '{}')
  } catch {
    marks.value = {}
  }
}
function toggleMark(id: number) {
  marks.value[id] = !marks.value[id]
  localStorage.setItem(marksKey, JSON.stringify(marks.value))
}

// ---------- 计时 ----------
function syncRemain() {
  if (!deadlineAt.value) {
    remainMs.value = 0
    return
  }
  remainMs.value = Math.max(0, deadlineAt.value - (Date.now() + serverOffset))
}
function formatHMS(totalSec: number) {
  const s = Math.max(0, totalSec)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`
}
function formatDur(sec: number) {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  return h > 0
    ? `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
    : `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

// ---------- 数据加载 ----------
async function loadPaper() {
  try {
    const p = await userApi.paper(quizId)
    paper.value = p
    submitted.value = p.submitted
    const a: Record<number, string[]> = {}
    for (const q of p.questions) a[q.id] = q.my_answer ? q.my_answer.split('') : []
    answers.value = a
    loadMarks()
    if (curIndex.value >= p.questions.length) curIndex.value = 0
  } catch {
    /* WAITING / 未加入等场景静默，等待 WS 事件驱动 */
  }
}

async function loadResult() {
  if (resultLoaded) return
  resultLoaded = true
  try {
    result.value = await userApi.result(quizId)
    const rk = await userApi.ranking(quizId)
    if (rk.visible && rk.items) finalRanking.value = rk.items
  } catch {
    resultLoaded = false
  }
}

// ---------- 作答：选择即保存（可修改，防抖 250ms） ----------
const qStartAt: Record<number, number> = {}
function toggle(label: string) {
  const q = curQ.value
  if (!q || submitted.value || status.value !== 'RUNNING') return
  if (!qStartAt[q.id]) qStartAt[q.id] = Date.now()
  const cur = answers.value[q.id] || []
  if (q.type === 'single' || q.type === 'judge') {
    answers.value[q.id] = [label]
  } else {
    answers.value[q.id] = cur.includes(label)
      ? cur.filter((x) => x !== label)
      : [...cur, label].sort()
  }
  saveState.value = 'saving'
  persist(q.id)
}

function persist(questionId: number) {
  const prev = saveTimers.get(questionId)
  if (prev) clearTimeout(prev)
  const t = window.setTimeout(async () => {
    saveTimers.delete(questionId)
    const ans = (answers.value[questionId] || []).join('')
    try {
      await userApi.savePaperAnswer(quizId, questionId, ans, Date.now() - (qStartAt[questionId] || Date.now()))
      saveState.value = 'saved'
      window.setTimeout(() => {
        if (saveState.value === 'saved') saveState.value = ''
      }, 1600)
    } catch (e: any) {
      saveState.value = ''
      toast(e?.response?.data?.msg || '保存失败，请重试')
    }
  }, 250)
  saveTimers.set(questionId, t)
}

// ---------- 切题 ----------
function go(i: number) {
  if (!paper.value) return
  if (i < 0 || i >= paper.value.questions.length) return
  curIndex.value = i
}

// ---------- 交卷 ----------
function askSubmit() {
  showSubmitModal.value = true
}
async function doSubmit() {
  if (submitting.value) return
  submitting.value = true
  try {
    await userApi.submitPaper(quizId)
    submitted.value = true
    showSubmitModal.value = false
    resultLoaded = false
    result.value = null
    await loadResult()
    toast('交卷成功，成绩已锁定')
  } catch (e: any) {
    toast(e?.response?.data?.msg || '交卷失败，请重试')
  } finally {
    submitting.value = false
  }
}

function exit() {
  localStorage.removeItem(LS.userToken(quizId))
  router.replace('/join')
}

function typeText(t: string) {
  return ({ single: '单选', multiple: '多选', judge: '判断' } as Record<string, string>)[t] || t
}

// ---------- WS ----------
function handleEvent(msg: WSMessage) {
  const d = msg.data || {}
  switch (msg.event) {
    case Ev.Sync:
      serverOffset = (d.server_now || Date.now()) - Date.now()
      status.value = d.status || 'WAITING'
      if (d.deadline_at) deadlineAt.value = d.deadline_at
      syncRemain()
      if (d.status && d.status !== 'WAITING') {
        loadPaper().then(() => {
          if (submitted.value || d.status === 'FINISHED') loadResult()
        })
      }
      break

    case Ev.ActivityStart:
      status.value = 'RUNNING'
      serverOffset = (d.server_now || Date.now()) - Date.now()
      if (d.deadline_at) deadlineAt.value = d.deadline_at
      syncRemain()
      loadPaper()
      break

    case Ev.QuestionCountdown:
      // 考试模式：QuestionID=0 的整体倒计时广播
      if (d.deadline_at) {
        deadlineAt.value = d.deadline_at
        syncRemain()
      } else if (d.remain_sec !== undefined) {
        remainMs.value = d.remain_sec * 1000
      }
      break

    case Ev.ActivityEnd:
      status.value = 'FINISHED'
      if (d.ranking) finalRanking.value = d.ranking
      loadResult()
      break

    case Ev.RankingUpdate:
      if (d.items) finalRanking.value = d.items
      break
  }
}

onMounted(async () => {
  const token = userApi.quizToken(quizId)
  if (!token) {
    router.replace('/join')
    return
  }
  nick.value = localStorage.getItem(LS.nickname(quizId)) || localStorage.getItem(LS.userNick) || '我'
  meId.value = Number(localStorage.getItem(LS.userId(quizId)) || 0)
  loadPaper() // 直接拉全卷（含已存答案），WS 只负责状态/倒计时
  loadMarks()

  ws = new QuizWS({
    token,
    onStatus: (s) => (wsStatus.value = s),
    onEvent: (msg: WSMessage) => handleEvent(msg),
  })
  tickTimer = window.setInterval(syncRemain, 250)
})

onUnmounted(() => {
  ws?.close()
  if (tickTimer) clearInterval(tickTimer)
  saveTimers.forEach((t) => clearTimeout(t))
})
</script>

<style scoped>
.exam {
  max-width: 1080px;
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
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: saturate(1.8) blur(16px);
  -webkit-backdrop-filter: saturate(1.8) blur(16px);
  border-bottom: 1px solid var(--border);
}
.bar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  gap: 12px;
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
.answered-chip {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-dim);
  background: var(--card-2);
  padding: 4px 12px;
  border-radius: 999px;
  white-space: nowrap;
}
.countdown {
  min-width: 96px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--primary);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  padding: 0 14px;
}
.countdown.urgent {
  background: #ff3b30;
  animation: blink 0.8s ease-in-out infinite;
}
@keyframes blink {
  50% { opacity: 0.75; }
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
  padding: 16px 20px 48px;
  width: 100%;
  margin: 0 auto;
}
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

.panel {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 24px;
  box-shadow: var(--shadow);
}

/* 等待页 */
.hero-panel {
  text-align: center;
  padding: 48px 24px;
  max-width: 680px;
  margin: 0 auto;
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
.pulse-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--primary);
  margin: 28px auto 12px;
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
.spinner {
  width: 34px;
  height: 34px;
  margin: 12px auto;
  border: 3px solid var(--card-2);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

/* 双栏布局：左题目 + 右索引（桌面） */
.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 16px;
  align-items: start;
}

/* 移动端索引折叠条（桌面隐藏） */
.idx-toggle {
  display: none;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  margin-bottom: 12px;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--card);
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
}
.idx-toggle i {
  font-style: normal;
  font-size: 12px;
  color: var(--text-dim);
}

/* 题目卡 */
.q-panel {
  padding: 26px 24px 20px;
}
.q-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  flex-wrap: wrap;
  align-items: center;
}
.tag {
  font-size: 12px;
  font-weight: 600;
  color: var(--primary);
  background: rgba(0, 113, 227, 0.08);
  border-radius: 999px;
  padding: 4px 12px;
}
.mark-btn {
  margin-left: auto;
  border: 1.5px solid var(--border);
  background: var(--card);
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  border-radius: 999px;
  padding: 4px 14px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.mark-btn.on {
  color: #fa8c16;
  border-color: rgba(250, 140, 22, 0.65);
  background: rgba(250, 140, 22, 0.08);
}
.q-content {
  font-size: 21px;
  font-weight: 650;
  line-height: 1.45;
  letter-spacing: -0.01em;
  margin-bottom: 22px;
}

/* 选项 */
.opts {
  display: flex;
  flex-direction: column;
  gap: 12px;
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
.opt:active {
  transform: scale(0.985);
}
.opt.sel {
  border-color: var(--primary);
  background: rgba(0, 113, 227, 0.06);
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
.opt-text {
  flex: 1;
  min-width: 0;
}
.save-hint {
  height: 18px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--success);
  text-align: right;
  opacity: 0;
  transition: opacity 0.2s ease;
}
.save-hint.show {
  opacity: 1;
}

/* 底部导航 */
.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 14px;
  padding-top: 16px;
  border-top: 1px dashed var(--border);
}
.nav-btn {
  min-width: 112px;
}
.nav-num {
  font-size: 15px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--text);
}
.nav-num i {
  font-style: normal;
  color: var(--text-dim);
  font-weight: 400;
  margin: 0 2px;
}

/* 右侧题目索引卡 */
.idx-panel {
  padding: 18px 16px;
  position: sticky;
  top: 76px;
}
.idx-title {
  font-size: 15px;
  font-weight: 700;
  margin-bottom: 12px;
}
.qix {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 8px;
}
.qix-box {
  position: relative;
  aspect-ratio: 1;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  font-family: inherit;
  color: var(--text-dim);
  background: var(--card);
  border: 1.5px solid var(--border);
  cursor: pointer;
  transition: all 0.15s ease;
}
.qix-box:hover {
  border-color: var(--primary);
  color: var(--primary);
}
/* 已答：蓝底白字 */
.qix-box.done {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
/* 已标记：橙色边框 + 橙色角标 */
.qix-box.mark {
  border-color: #fa8c16;
}
.qix-box.mark::after {
  content: '';
  position: absolute;
  right: 3px;
  top: 3px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #fa8c16;
}
.qix-box.mark:not(.done) {
  color: #fa8c16;
}
/* 当前题：蓝色高亮描边 */
.qix-box.cur {
  box-shadow: 0 0 0 3px rgba(0, 113, 227, 0.28);
  transform: scale(1.06);
}
.qix-box.cur:not(.done) {
  border-color: var(--primary);
  color: var(--primary);
}

/* 状态说明 */
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin: 14px 0;
  font-size: 12px;
  color: var(--text-dim);
}
.legend span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.lg {
  width: 12px;
  height: 12px;
  border-radius: 4px;
  display: inline-block;
}
.lg-todo {
  background: var(--card);
  border: 1.5px solid var(--border);
}
.lg-done {
  background: var(--primary);
}
.lg-cur {
  background: var(--card);
  border: 1.5px solid var(--primary);
  box-shadow: 0 0 0 2px rgba(0, 113, 227, 0.25);
}
.lg-mark {
  background: var(--card);
  border: 1.5px solid #fa8c16;
}

/* 交卷按钮（红色强调） */
.submit-all {
  width: 100%;
  border: none;
  border-radius: 14px;
  padding: 13px;
  font-size: 16px;
  font-weight: 700;
  font-family: inherit;
  color: #fff;
  background: #ff3b30;
  cursor: pointer;
  box-shadow: 0 6px 18px rgba(255, 59, 48, 0.32);
  transition: transform 0.12s ease, box-shadow 0.2s ease;
}
.submit-all:hover {
  box-shadow: 0 8px 24px rgba(255, 59, 48, 0.42);
}
.submit-all:active {
  transform: scale(0.98);
}

/* 交卷确认弹窗 */
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 20px;
}
.modal {
  width: 100%;
  max-width: 400px;
  padding: 26px 24px 20px;
  border-radius: 22px;
}
.modal h3 {
  font-size: 19px;
  font-weight: 700;
  margin-bottom: 10px;
}
.modal-msg {
  font-size: 14px;
  color: var(--text-dim);
  line-height: 1.6;
  margin-bottom: 20px;
}
.modal-msg .t-bad {
  color: #ff3b30;
  font-size: 16px;
}
.modal-actions {
  display: flex;
  gap: 10px;
}
.modal-actions .btn {
  flex: 1;
}
.btn {
  border: none;
  border-radius: 14px;
  padding: 12px 18px;
  font-size: 15px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: transform 0.1s ease, opacity 0.15s ease;
}
.btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.btn-ghost {
  background: var(--card-2);
  color: var(--text);
}
.btn-danger {
  background: #ff3b30;
  color: #fff;
  box-shadow: 0 6px 18px rgba(255, 59, 48, 0.32);
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
.text-dim {
  color: var(--text-dim);
}
.exit-btn {
  margin-top: 20px;
}

/* 移动端：单列 + 索引折叠 */
@media (max-width: 960px) {
  .body {
    padding: 12px 14px 40px;
  }
  .cols {
    grid-template-columns: 1fr;
  }
  .idx-toggle {
    display: flex;
  }
  .idx-panel {
    display: none;
  }
  .idx-panel.open {
    display: block;
    position: static;
    margin-bottom: 12px;
  }
  .idx-panel.open .qix {
    max-height: 42vh;
    overflow: auto;
  }
  .q-panel {
    padding: 20px 16px 16px;
  }
  .q-content {
    font-size: 19px;
  }
  .opt {
    font-size: 15px;
    padding: 14px;
  }
  .countdown {
    min-width: 84px;
    font-size: 14px;
    height: 32px;
  }
  .answered-chip {
    font-size: 12px;
    padding: 3px 10px;
  }
  .nav-btn {
    min-width: 96px;
    padding: 10px 12px;
    font-size: 14px;
  }
  .result-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .final-score {
    font-size: 60px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .pulse-dot, .countdown.urgent, .spinner {
    animation: none;
  }
}
</style>
