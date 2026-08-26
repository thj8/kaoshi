<template>
  <div class="stage">
    <div class="panel">
      <!-- 卡片头 -->
      <header class="p-head">
        <div>
          <h1><span class="live-dot" :class="connClass"></span>理论赛实时排行榜</h1>
          <p class="p-sub">{{ teams.length }} 支队伍 · 实时更新</p>
        </div>
        <span class="pill" :class="connClass">{{ connText }}</span>
      </header>

      <!-- 列头（桌面） -->
      <div v-if="teams.length" class="cols">
        <span>排名</span><span>队伍</span><span class="r">总分</span>
        <span class="r">必答分</span><span class="r">抢答分</span>
        <span class="r">答题数</span><span class="r">答对数</span><span class="r">正确率</span><span class="r">更新时间</span>
      </div>

      <!-- 榜单 -->
      <transition-group name="rk" tag="div" class="list">
        <div
          v-for="t in teams"
          :key="t.id"
          class="row"
          :class="[`p${Math.min(t.rank, 3)}`, { flashUp: t.fx === 'up', flashDown: t.fx === 'down' }]"
        >
          <div class="rk"><i>{{ medal(t.rank) }}</i><b>{{ t.rank }}</b></div>
          <div class="nm">
            <b>{{ t.name }}</b>
            <transition name="pop">
              <span v-if="t.delta !== 0" class="delta" :class="t.delta > 0 ? 'up' : 'down'">
                {{ t.delta > 0 ? '↑' : '↓' }}{{ Math.abs(t.delta) }}
              </span>
            </transition>
          </div>
          <div class="sc">
            <b>{{ t.disp }}</b><em>分</em>
            <transition name="pop">
              <span v-if="t.diff !== 0" class="diff" :class="t.diff > 0 ? 'up' : 'down'">
                {{ t.diff > 0 ? '+' : '' }}{{ t.diff }}
              </span>
            </transition>
          </div>
          <div class="mini r gold">{{ t.reqScore }}</div>
          <div class="mini r cyan">{{ t.rushScore }}</div>
          <div class="mini r">{{ t.answered }}</div>
          <div class="mini r ok">{{ t.correct }}</div>
          <div class="mini r dim">{{ rate(t) }}</div>
          <div class="time r">{{ t.updatedAt ? fmt(t.updatedAt) : '--:--:--' }}</div>
          <i class="bar" :style="{ width: barW(t) + '%' }"></i>
        </div>
      </transition-group>

      <p v-if="teams.length === 0" class="empty">{{ hint || '暂无最新数据' }}</p>
    </div>

    <!-- 得分事件 Toast -->
    <div class="toasts">
      <transition-group name="ts">
        <div v-for="t in toasts" :key="t.id" class="toast">{{ t.text }}</div>
      </transition-group>
    </div>

    <!-- 演示控制器 -->
    <div v-if="demo" class="mock-bar">
      <button @click="mock('代码先锋队', 5)">代码先锋队 +5</button>
      <button @click="mock('代码先锋队', -3)">代码先锋队 -3</button>
      <button @click="mock('Cyber守护者', 10)">Cyber守护者 +10</button>
      <button @click="mock('智算未来队', 8)">智算未来队 +8</button>
      <button @click="mockRandom(1)">随机得分</button>
      <button @click="mockRandom(-1)">随机扣分</button>
      <button @click="mockSwap()">触发排名变化</button>
      <button @click="resetMock()">恢复初始</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { QuizWS } from '../ws'
import { userApi } from '../api/user'

interface Team {
  id: number
  name: string
  score: number // 真实分
  disp: number // 滚动显示分
  answered: number
  correct: number
  reqScore: number
  rushScore: number
  rank: number
  prevRank: number
  delta: number // 排名变化（正=上升）
  diff: number // 得分变化
  fx: '' | 'up' | 'down'
  updatedAt: number
  timer?: number
  tween?: number
}

const route = useRoute()
const quizId = Number(route.params.id)
const demo = route.query.demo === '1'
const teams = reactive<Team[]>([])
const hint = ref('')
const wsStatus = ref<'connecting' | 'open' | 'closed' | 'retrying' | ''>(demo ? '' : 'connecting')

const byId = new Map<number, Team>()
let ws: QuizWS | null = null

const DEMO = [
  ['代码先锋队', 83, 45, 40], ['智算未来队', 76, 42, 36], ['Cyber守护者', 72, 38, 32],
  ['网安小分队', 68, 36, 28], ['数据护盾队', 64, 33, 26], ['漏洞猎手队', 58, 30, 24],
  ['安全研究院', 52, 28, 20], ['知识圈团队', 46, 26, 18], ['二进制战队', 40, 22, 16], ['算法艺术家', 33, 20, 12],
] as const

function seedDemo() {
  teams.length = 0
  byId.clear()
  DEMO.forEach(([name, score, ans, ok], i) => {
    const t = reactive<Team>({ id: i + 1, name, score, disp: score, answered: ans, correct: ok, reqScore: Math.round(score * 0.7), rushScore: score - Math.round(score * 0.7), rank: i + 1, prevRank: i + 1, delta: 0, diff: 0, fx: '', updatedAt: Date.now() })
    teams.push(t); byId.set(t.id, t)
  })
}

/** 核心更新入口：diffs = {id: {score, answered, correct}}，只更新变化的队伍 */
function applyUpdate(rows: { user_id?: number; nickname?: string; score: number; answered: number; correct: number; required_score?: number; rush_score?: number }[], ids: number[] = []) {
  const prevOrder = new Map(teams.map((t) => [t.id, t.rank]))
  for (const r of rows) {
    const id = r.user_id ?? ids.shift()!
    let t = byId.get(id)
    if (!t) {
      t = reactive<Team>({ id, name: r.nickname || `队伍${id}`, score: 0, disp: 0, answered: 0, correct: 0, reqScore: 0, rushScore: 0, rank: 99, prevRank: 99, delta: 0, diff: 0, fx: '', updatedAt: 0 })
      teams.push(t); byId.set(id, t)
    }
    const dScore = r.score - t.score
    if (r.nickname) t.name = r.nickname
    t.answered = r.answered; t.correct = r.correct
    t.reqScore = r.required_score ?? 0; t.rushScore = r.rush_score ?? (t.score - (r.required_score ?? 0))
    if (dScore !== 0) {
      t.score = r.score
      t.diff = dScore
      t.updatedAt = Date.now()
      roll(t)
      toast(`${t.name} ${dScore > 0 ? '+' : ''}${dScore}`)
      clearTimeout(t.timer)
      t.timer = window.setTimeout(() => (t.diff = 0), 1100)
    } else {
      t.score = r.score
    }
  }
  // 重排 + 排名变化标记
  teams.sort((a, b) => b.score - a.score || b.correct - a.correct)
  teams.forEach((t, i) => {
    t.prevRank = prevOrder.get(t.id) ?? i + 1
    t.rank = i + 1
    const d = t.prevRank - t.rank
    if (d !== 0) {
      t.delta = d
      t.fx = d > 0 ? 'up' : 'down'
      clearTimeout(t.timer)
      t.timer = window.setTimeout(() => { t.delta = 0; t.fx = '' }, 1200)
    }
  })
}

/** 数字滚动动画（500-800ms） */
function roll(t: Team) {
  cancelAnimationFrame(t.tween!)
  const from = t.disp, to = t.score, t0 = performance.now(), dur = 650
  const step = (now: number) => {
    const p = Math.min(1, (now - t0) / dur)
    t.disp = Math.round(from + (to - from) * (1 - Math.pow(1 - p, 3)))
    if (p < 1) t.tween = requestAnimationFrame(step)
  }
  t.tween = requestAnimationFrame(step)
}

// ---- 右上角得分事件 Toast ----
const toasts = reactive<{ id: number; text: string }[]>([])
let toastSeq = 0
function toast(text: string) {
  if (toasts.length > 3) toasts.shift()
  const item = { id: ++toastSeq, text }
  toasts.push(item)
  setTimeout(() => {
    const i = toasts.indexOf(item)
    if (i >= 0) toasts.splice(i, 1)
  }, 1800)
}

// ---- 演示模式 ----
function mock(name: string, diff: number) {
  const t = teams.find((x) => x.name === name)
  if (!t) return
  applyUpdate([{ user_id: t.id, score: t.score + diff, answered: t.answered, correct: t.correct }])
}
function mockRandom(sign: number) {
  const t = teams[Math.floor(Math.random() * teams.length)]
  mock(t.name, sign * (1 + Math.floor(Math.random() * 8)))
}
function mockSwap() {
  const first = teams[0]
  const target = teams[Math.floor(teams.length / 2)]
  mock(target.name, first.score - target.score + 3)
}
function resetMock() { seedDemo() }

// ---- 真实数据 ----
const connClass = computed(() => (demo ? 'ok' : wsStatus.value === 'open' ? 'ok' : wsStatus.value === 'retrying' ? 'warn' : 'off'))
const connText = computed(() => (demo ? '演示模式' : wsStatus.value === 'open' ? '实时更新' : wsStatus.value === 'retrying' ? '正在重新连接…' : wsStatus.value === 'connecting' ? '连接中…' : '暂无最新数据'))

async function connect() {
  const token = userApi.quizToken(quizId)
  if (!token) {
    hint.value = '请先从选手页加入比赛后，再打开排行榜'
    wsStatus.value = 'closed'
    return
  }
  try {
    const res = await userApi.ranking(quizId)
    const items = res?.items
    if (items?.length) applyUpdate(items.map((r: any) => ({ ...r })))
  } catch { /* 首屏拉取失败没关系，靠 WS 推 */ }
  ws = new QuizWS({
    token,
    onStatus: (s) => (wsStatus.value = s),
    onEvent: (msg) => {
      if (msg.event === 'ranking:update' && msg.data?.items) {
        applyUpdate(msg.data.items.map((i: any) => ({ ...i })))
      }
    },
  })
}

onMounted(() => { demo ? seedDemo() : connect() })
onBeforeUnmount(() => ws?.close())

const medal = (r: number) => (r === 1 ? '🥇' : r === 2 ? '🥈' : r === 3 ? '🥉' : '')
const rate = (t: Team) => (t.answered > 0 ? ((t.correct / t.answered) * 100).toFixed(1) + '%' : '—')
const fmt = (ms: number) => new Date(ms).toTimeString().slice(0, 8)
const barW = (t: Team) => {
  const max = Math.max(100, ...teams.map((x) => x.score))
  return Math.min(100, (t.score / max) * 100)
}
</script>

<style scoped>
.stage {
  min-height: 100vh;
  background:
    radial-gradient(1000px 520px at 20% -8%, rgba(22, 119, 255, 0.16), transparent 62%),
    radial-gradient(760px 420px at 88% 110%, rgba(22, 119, 255, 0.10), transparent 60%),
    linear-gradient(180deg, #071428 0%, #050e1e 60%, #040a16 100%);
  padding: clamp(16px, 3.4vw, 48px);
  font-variant-numeric: tabular-nums;
}

/* ---- 深蓝玻璃榜单卡 ---- */
.panel {
  width: min(96%, 1560px);
  margin: 0 auto;
  border-radius: 24px;
  padding: clamp(20px, 2.6vw, 40px) clamp(18px, 2.6vw, 44px) clamp(22px, 2.6vw, 36px);
  background:
    radial-gradient(120% 90% at 100% 0%, rgba(38, 108, 217, 0.35), transparent 55%),
    linear-gradient(165deg, #0d2c5a 0%, #081f42 55%, #061733 100%);
  border: 1px solid rgba(140, 170, 220, 0.16);
  border-color: rgba(120, 165, 235, 0.22);
  box-shadow:
    0 34px 90px rgba(2, 8, 20, 0.65),
    inset 0 1px 0 rgba(160, 200, 255, 0.12);
  position: relative;
  overflow: hidden;
}
.panel::before {
  /* 顶部细高光线，玻璃质感 */
  content: '';
  position: absolute; inset: 0 0 auto 0; height: 1px;
  background: linear-gradient(90deg, transparent, rgba(160, 200, 255, 0.55), transparent);
}

.p-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 26px; }
.p-head h1 {
  display: flex; align-items: center; gap: 12px;
  color: #fff; font-size: clamp(19px, 2.1vw, 30px); font-weight: 800; letter-spacing: 3px; margin: 0;
}
.p-sub { margin: 8px 0 0 34px; color: #7f95b8; font-size: 13px; letter-spacing: 1px; }
.live-dot { width: 10px; height: 10px; border-radius: 50%; flex: none; }
.live-dot.ok { background: #52ff7e; box-shadow: 0 0 10px #52c41a; animation: pulse 1.6s infinite; }
.live-dot.warn { background: #faad14; box-shadow: 0 0 10px #faad14; }
.live-dot.off { background: #5b6b85; }
@keyframes pulse { 50% { opacity: 0.45; } }

.pill {
  flex: none; color: #9fc2ff; font-size: 13px; letter-spacing: 1px;
  border: 1px solid rgba(120, 165, 235, 0.35); background: rgba(22, 119, 255, 0.12);
  padding: 7px 16px; border-radius: 999px; white-space: nowrap;
}
.pill.warn { color: #ffd666; border-color: rgba(250, 173, 20, 0.45); background: rgba(250, 173, 20, 0.10); }
.pill.off { color: #8b9bb5; border-color: rgba(139, 155, 181, 0.35); background: rgba(139, 155, 181, 0.08); }

/* ---- 列头 ---- */
.cols, .row {
  display: grid;
  grid-template-columns: 76px minmax(150px, 1.15fr) 150px repeat(5, minmax(58px, 0.55fr)) 104px;
  align-items: center;
  column-gap: 10px;
}
.cols {
  color: #6d84a8; font-size: 12.5px; letter-spacing: 2px;
  padding: 0 22px 10px;
}
.r { text-align: right; }

/* ---- 行卡片 ---- */
.list { position: relative; display: flex; flex-direction: column; gap: 10px; }
.row {
  position: relative;
  min-height: 68px;
  padding: 10px 22px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.045);
  border: 1px solid rgba(148, 180, 230, 0.13);
  transition: transform 0.55s cubic-bezier(0.22, 0.61, 0.36, 1), box-shadow 0.35s, border-color 0.35s, background 0.35s;
  overflow: hidden;
}
.rk-move { transition: transform 0.55s cubic-bezier(0.22, 0.61, 0.36, 1); }
.rk-enter-active { transition: all 0.4s; }
.rk-enter-from { opacity: 0; transform: translateY(14px); }
.rk-leave-active { display: none; }

/* 分数进度条：行底 */
.bar {
  position: absolute; left: 0; bottom: 0; height: 3px;
  background: linear-gradient(90deg, #1677ff, #69b4ff);
  opacity: 0.75; transition: width 0.65s ease-out; pointer-events: none;
}

/* 前三名 */
.row.p1 {
  background: linear-gradient(90deg, rgba(255, 205, 60, 0.13), rgba(255, 255, 255, 0.04));
  border-color: rgba(255, 215, 0, 0.45);
  box-shadow: 0 0 26px rgba(255, 200, 40, 0.16);
}
.row.p1 .bar { background: linear-gradient(90deg, #ffd700, #fff3b0); }
.row.p2 { background: linear-gradient(90deg, rgba(192, 198, 210, 0.12), rgba(255, 255, 255, 0.04)); border-color: rgba(200, 210, 225, 0.38); }
.row.p3 { background: linear-gradient(90deg, rgba(205, 127, 50, 0.13), rgba(255, 255, 255, 0.04)); border-color: rgba(205, 127, 50, 0.4); }

/* 事件高光 */
.row.flashUp { border-color: rgba(82, 196, 26, 0.75); box-shadow: 0 0 22px rgba(82, 196, 26, 0.35); }
.row.flashDown { border-color: rgba(255, 77, 79, 0.75); box-shadow: 0 0 22px rgba(255, 77, 79, 0.35); }

/* 名次徽章 */
.rk { display: flex; align-items: center; gap: 6px; }
.rk i { font-style: normal; font-size: 21px; width: 24px; text-align: center; }
.rk b {
  width: 38px; height: 38px; border-radius: 12px;
  display: grid; place-items: center;
  font-size: 18px; font-weight: 800; color: #cfe2ff;
  background: rgba(22, 119, 255, 0.16);
  border: 1px solid rgba(105, 180, 255, 0.28);
}
.row.p1 .rk b { color: #ffd700; background: rgba(255, 215, 0, 0.14); border-color: rgba(255, 215, 0, 0.5); box-shadow: 0 0 14px rgba(255, 215, 0, 0.25); }
.row.p2 .rk b { color: #dde3ec; background: rgba(200, 208, 222, 0.13); border-color: rgba(200, 210, 225, 0.45); }
.row.p3 .rk b { color: #e8a468; background: rgba(205, 127, 50, 0.15); border-color: rgba(205, 127, 50, 0.5); }

/* 队伍名 */
.nm { display: flex; align-items: center; gap: 10px; min-width: 0; }
.nm b { color: #fff; font-size: clamp(15px, 1.35vw, 20px); font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.row.p1 .nm b { color: #ffe27a; }

.delta { flex: none; font-size: 13px; font-weight: 800; line-height: 1; padding: 4px 8px; border-radius: 999px; }
.delta.up { color: #6fe08a; background: rgba(82, 196, 26, 0.16); }
.delta.down { color: #ff8f92; background: rgba(255, 77, 79, 0.16); }

/* 总分 */
.sc { display: flex; align-items: baseline; justify-content: flex-end; gap: 4px; }
.sc b { color: #fff; font-weight: 800; font-size: clamp(22px, 1.9vw, 30px); line-height: 1; }
.sc em { font-style: normal; color: #7f95b8; font-size: 12px; }
.row.p1 .sc b { color: #ffd700; text-shadow: 0 0 18px rgba(255, 215, 0, 0.45); }
.diff { flex: none; font-size: 15px; font-weight: 800; margin-left: 4px; }
.diff.up { color: #52ff7e; text-shadow: 0 0 12px rgba(82, 196, 26, 0.6); }
.diff.down { color: #ff6b6e; text-shadow: 0 0 12px rgba(255, 77, 79, 0.6); }

.mini { color: #dbe6f5; font-size: clamp(14px, 1.15vw, 17px); font-weight: 600; }
.mini.ok { color: #8ee6a1; }
.mini.dim { color: #9fb3cf; }
.mini.gold { color: #ffd700; }
.mini.cyan { color: #6fd4ff; }
.time { color: #6d84a8; font-size: 12.5px; }

.pop-enter-active, .pop-leave-active { transition: all 0.25s; }
.pop-enter-from, .pop-leave-to { opacity: 0; transform: scale(0.7); }

.empty { text-align: center; color: #8b9bb5; padding: 70px 0; font-size: 15px; letter-spacing: 1px; }

/* ---- Toast ---- */
.toasts { position: fixed; top: 20px; right: 20px; display: flex; flex-direction: column; gap: 8px; z-index: 30; }
.toast {
  color: #fff; font-size: 14px; font-weight: 700; letter-spacing: 0.5px;
  background: rgba(8, 26, 56, 0.92); border: 1px solid rgba(105, 180, 255, 0.4);
  padding: 10px 18px; border-radius: 12px; backdrop-filter: blur(6px);
  box-shadow: 0 10px 28px rgba(6, 23, 51, 0.45);
}
.ts-enter-active, .ts-leave-active { transition: all 0.3s; }
.ts-enter-from { opacity: 0; transform: translateX(36px); }
.ts-leave-to { opacity: 0; transform: translateY(-8px); }

/* ---- 演示控制器 ---- */
.mock-bar { position: fixed; left: 16px; bottom: 16px; display: flex; flex-wrap: wrap; gap: 8px; max-width: 430px; z-index: 30; }
.mock-bar button {
  background: rgba(13, 44, 90, 0.92); color: #9fc2ff; cursor: pointer;
  border: 1px solid rgba(105, 180, 255, 0.35); border-radius: 9px;
  padding: 7px 11px; font-size: 12px;
}
.mock-bar button:hover { background: rgba(22, 119, 255, 0.35); color: #fff; }

/* ---- 平板 ---- */
@media (max-width: 1279px) {
  .cols, .row { grid-template-columns: 60px minmax(130px, 1.2fr) 118px repeat(4, minmax(48px, 0.55fr)); }
  .cols span:nth-child(8), .cols span:nth-child(9),
  .row .mini:nth-child(8), .time { display: none; }
}

/* ---- 手机卡片 ---- */
@media (max-width: 767px) {
  .stage { padding: 12px; }
  .panel { border-radius: 18px; padding: 18px 14px; }
  .cols { display: none; }
  .list { gap: 9px; }
  .row {
    grid-template-columns: 54px 1fr auto;
    grid-template-areas: 'rk nm sc' 'st st st';
    row-gap: 7px; padding: 12px 14px; min-height: 0;
  }
  .rk { grid-area: rk; }
  .rk b { width: 32px; height: 32px; font-size: 15px; border-radius: 10px; }
  .rk i { font-size: 17px; width: 20px; }
  .nm { grid-area: nm; }
  .nm b { font-size: 16px; }
  .sc { grid-area: sc; }
  .sc b { font-size: 24px; }
  .mini { font-size: 12.5px; font-weight: 500; }
  .mini.r { text-align: left; }
  .mini:nth-child(4) { grid-area: st; }
  .mini:nth-child(4)::before { content: '必答 '; color: #6d84a8; }
  .mini:nth-child(5), .mini:nth-child(6), .mini:nth-child(7) { display: inline-block; margin-left: 10px; }
  .mini:nth-child(5)::before { content: '· 抢答 '; color: #6d84a8; }
  .mini:nth-child(6)::before { content: '· 答题 '; color: #6d84a8; }
  .mini:nth-child(7)::before { content: '· 答对 '; color: #6d84a8; }
  .mini:nth-child(8), .time { display: none; }
}
</style>
