<template>
  <div class="stage">
    <div class="panel">
      <!-- 卡片头 -->
      <header class="p-head">
        <div>
          <h1><span class="live-dot" :class="polling ? 'on' : ''"></span>{{ title || '答题排行榜' }}</h1>
          <p class="p-sub">{{ st?.participants ?? 0 }} 名参赛 · 已交卷 {{ st?.finished ?? 0 }} · 每 5 秒自动刷新</p>
        </div>
        <div class="head-right">
          <span class="pill" :class="'st-' + (st?.status || '')">{{ statusText }}</span>
          <button class="btn-close" @click="close">关闭</button>
        </div>
      </header>

      <!-- 概况指标 -->
      <div class="chips">
        <div class="chip"><b>{{ st?.max_score ?? 0 }}</b><small>最高分</small></div>
        <div class="chip"><b>{{ st?.min_score ?? 0 }}</b><small>最低分</small></div>
        <div class="chip"><b>{{ fmt1(st?.avg_score) }}</b><small>平均分</small></div>
        <div class="chip"><b>{{ (st?.avg_correct_rate ?? 0).toFixed(0) }}%</b><small>平均正确率</small></div>
        <div class="chip"><b>{{ updatedAt }}</b><small>更新于</small></div>
      </div>

      <!-- 列头（桌面） -->
      <div v-if="rows.length" class="cols">
        <span>排名</span><span>参赛者</span><span class="r">得分</span>
        <span class="r">答对</span><span class="r">答错</span><span class="r">正确率</span><span class="r">交卷时间</span>
      </div>

      <!-- 榜单 -->
      <div class="list">
        <div v-for="r in rows" :key="r.user_id" class="row" :class="`p${Math.min(r.rank, 3)}`">
          <div class="rk"><i>{{ medal(r.rank) }}</i><b>{{ r.rank }}</b></div>
          <div class="nm"><b>{{ r.nickname }}</b></div>
          <div class="sc r">
            <b>{{ r.score }}</b><em>分</em>
            <i class="bar" :style="{ width: barW(r.score) + '%' }"></i>
          </div>
          <div class="mini r ok">{{ r.correct }}</div>
          <div class="mini r no">{{ r.wrong }}</div>
          <div class="mini r dim">{{ rate(r) }}</div>
          <div class="mini r tm" :class="{ na: !r.submitted_at }">{{ fmtT(r.submitted_at) }}</div>
        </div>
      </div>

      <p v-if="!rows.length" class="empty">{{ st ? '暂无排名数据' : '加载中…' }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { adminApi, type Statistics } from '../api/admin'

const route = useRoute()
const quizId = String(route.params.id || '')

const title = ref('')
const st = ref<Statistics | null>(null)
const updatedAt = ref('--:--:--')
const polling = ref(false)
let timer: number | null = null

const rows = computed(() => st.value?.ranking ?? [])
const statusText = computed(
  () => ({ WAITING: '未开始', RUNNING: '进行中', ANSWERING: '进行中', REVEALING: '公布中', PAUSED: '已暂停', FINISHED: '已结束' }[st.value?.status || ''] || st.value?.status || '')
)

async function load() {
  try {
    st.value = await adminApi.statistics(quizId)
    updatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } catch {
    /* 静默：下一轮重试 */
  }
}

function medal(rank: number) {
  return rank === 1 ? '🥇' : rank === 2 ? '🥈' : rank === 3 ? '🥉' : ''
}
const maxScore = computed(() => Math.max(1, ...rows.value.map((r) => r.score)))
function barW(score: number) {
  return Math.max(4, (score / maxScore.value) * 100)
}
function rate(r: { correct: number; wrong: number }) {
  const total = r.correct + r.wrong
  return total > 0 ? Math.round((r.correct / total) * 100) + '%' : '—'
}
function fmt1(v?: number) {
  return (v ?? 0).toFixed(1)
}
/** 交卷时间：HH:MM:SS；未交卷显示 — */
function fmtT(ms?: number) {
  if (!ms) return '—'
  return new Date(ms).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
function close() {
  window.close()
  window.history.length > 1 ? window.history.back() : window.close()
}

onMounted(async () => {
  try {
    const q = await adminApi.getQuiz(quizId)
    title.value = q.quiz.title
  } catch { /* 标题加载失败不阻塞榜单 */ }
  await load()
  polling.value = true
  timer = window.setInterval(load, 5000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  polling.value = false
})
</script>

<style scoped>
.stage {
  min-height: 100vh;
  background:
    radial-gradient(1000px 520px at 20% -8%, rgba(22, 119, 255, 0.16), transparent 62%),
    radial-gradient(760px 420px at 88% 110%, rgba(22, 119, 255, 0.1), transparent 60%),
    linear-gradient(180deg, #071428 0%, #050e1e 60%, #040a16 100%);
  padding: clamp(16px, 3.4vw, 48px);
  font-variant-numeric: tabular-nums;
}
.panel {
  width: min(96%, 1360px);
  margin: 0 auto;
  border-radius: 24px;
  padding: clamp(20px, 2.6vw, 40px) clamp(18px, 2.6vw, 44px);
  background:
    radial-gradient(120% 90% at 100% 0%, rgba(38, 108, 217, 0.35), transparent 55%),
    linear-gradient(165deg, #0d2c5a 0%, #081f42 55%, #061733 100%);
  border: 1px solid rgba(140, 170, 220, 0.16);
  box-shadow: 0 34px 90px rgba(2, 8, 20, 0.65), inset 0 1px 0 rgba(160, 200, 255, 0.12);
  position: relative;
  overflow: hidden;
}
.panel::before {
  content: '';
  position: absolute;
  inset: 0 0 auto 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(160, 200, 255, 0.55), transparent);
}

.p-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.p-head h1 {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #fff;
  font-size: clamp(19px, 2.1vw, 28px);
  font-weight: 800;
  letter-spacing: 2px;
  margin: 0;
}
.p-sub {
  margin: 8px 0 0 22px;
  color: #7f95b8;
  font-size: 13px;
  letter-spacing: 1px;
}
.head-right {
  display: flex;
  align-items: center;
  gap: 10px;
}
.live-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex: none;
  background: #4a5a75;
}
.live-dot.on {
  background: #3ddc84;
  box-shadow: 0 0 0 0 rgba(61, 220, 132, 0.6);
  animation: pulse 2s infinite;
}
@keyframes pulse {
  70% { box-shadow: 0 0 0 12px rgba(61, 220, 132, 0); }
  100% { box-shadow: 0 0 0 0 rgba(61, 220, 132, 0); }
}
.pill {
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 700;
  color: #cfe0ff;
  background: rgba(90, 140, 220, 0.16);
  border: 1px solid rgba(140, 170, 220, 0.28);
  white-space: nowrap;
}
.pill.st-RUNNING, .pill.st-ANSWERING, .pill.st-REVEALING {
  color: #7ef0b2;
  background: rgba(61, 220, 132, 0.12);
  border-color: rgba(61, 220, 132, 0.35);
}
.pill.st-FINISHED {
  color: #ffd479;
  background: rgba(250, 173, 20, 0.1);
  border-color: rgba(250, 173, 20, 0.35);
}
.btn-close {
  border: 1px solid rgba(140, 170, 220, 0.28);
  background: transparent;
  color: #cfe0ff;
  border-radius: 10px;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
}
.btn-close:hover { background: rgba(140, 170, 220, 0.12); }

/* 概况指标 */
.chips {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-bottom: 22px;
}
.chip {
  border-radius: 14px;
  padding: 12px 16px;
  background: rgba(10, 32, 66, 0.6);
  border: 1px solid rgba(140, 170, 220, 0.14);
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.chip b {
  color: #fff;
  font-size: clamp(17px, 1.8vw, 24px);
  font-weight: 800;
}
.chip small {
  color: #7f95b8;
  font-size: 12px;
  letter-spacing: 1px;
}

/* 榜单 */
.cols,
.row {
  display: grid;
  grid-template-columns: 76px 1fr 190px 80px 80px 80px 110px;
  align-items: center;
  gap: 12px;
}
.cols {
  color: #6d84a8;
  font-size: 12px;
  letter-spacing: 2px;
  padding: 0 14px 10px;
  border-bottom: 1px solid rgba(140, 170, 220, 0.14);
}
.r { text-align: right; }
.row {
  position: relative;
  padding: 12px 14px;
  border-radius: 14px;
  border-bottom: 1px solid rgba(140, 170, 220, 0.08);
  overflow: hidden;
}
.row:hover { background: rgba(38, 108, 217, 0.08); }
.rk {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #cfe0ff;
}
.rk i { font-style: normal; font-size: 18px; }
.rk b { font-size: 16px; color: #9fb4d8; }
.row.p1 .rk b, .row.p2 .rk b, .row.p3 .rk b { color: #fff; font-size: 18px; }
.nm {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.nm b { color: #e8f1ff; font-size: 15px; }
.sc {
  position: relative;
  color: #fff;
  font-weight: 800;
  font-size: 18px;
}
.sc em {
  font-style: normal;
  font-size: 12px;
  font-weight: 400;
  color: #7f95b8;
  margin-left: 4px;
}
.bar {
  position: absolute;
  left: 0;
  bottom: 0;
  height: 3px;
  border-radius: 3px;
  background: linear-gradient(90deg, #2f7bff, #6c8cff);
  transition: width 0.6s ease;
}
.row.p1 .bar { background: linear-gradient(90deg, #ffd700, #ffec8a); }
.row.p2 .bar { background: linear-gradient(90deg, #c0c8d8, #eef2fa); }
.row.p3 .bar { background: linear-gradient(90deg, #cd7f32, #f0b080); }
.mini { color: #cfe0ff; font-size: 14px; }
.mini.ok { color: #7ef0b2; }
.mini.no { color: #ff9b8a; }
.mini.dim { color: #7f95b8; }
.tm { font-size: 13px; }
.tm.na { color: #5a7096; }
.empty {
  text-align: center;
  color: #7f95b8;
  padding: 40px 0 20px;
  letter-spacing: 2px;
}

@media (max-width: 760px) {
  .cols { display: none; }
  .row { grid-template-columns: 52px 1fr 100px 56px 56px; }
  .row > .mini.dim, .row > .tm { display: none; }
  .cols > .r { display: none; }
  .cols > .r:nth-child(3) { display: block; }
  .chips { grid-template-columns: repeat(3, 1fr); }
  .sc { font-size: 16px; }
}
</style>
