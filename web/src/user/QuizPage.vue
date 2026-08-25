<template>
  <div class="page" style="max-width: 640px; padding-top: 8vh">
    <!-- 连接状态条 -->
    <div v-if="store.wsStatus !== 'open'" class="card" style="padding: 10px 16px; margin-bottom: 14px; text-align: center; font-size: 14px">
      <span v-if="store.wsStatus === 'retrying'" style="color: var(--warn)">⚠️ 连接断开，正在重连...</span>
      <span v-else style="color: var(--text-dim)">连接中...</span>
    </div>

    <div class="card" style="text-align: center; padding: 40px 24px">
      <h1 style="font-size: 24px; margin-bottom: 10px">{{ store.quiz?.title || '加载中...' }}</h1>
      <p class="text-dim" style="margin-bottom: 28px">{{ store.quiz?.description || '' }}</p>

      <div style="display: flex; justify-content: center; gap: 28px; margin-bottom: 32px">
        <div>
          <div style="font-size: 26px; font-weight: 800">{{ store.quiz?.participant_count ?? '—' }}</div>
          <div class="text-dim" style="font-size: 12px">参与人数</div>
        </div>
        <div>
          <div style="font-size: 26px; font-weight: 800">{{ statusText }}</div>
          <div class="text-dim" style="font-size: 12px">状态</div>
        </div>
      </div>

      <div v-if="store.status === 'WAITING'" style="color: var(--primary)">
        <div class="pulse-dot"></div>
        <p style="margin-top: 12px">等待管理员开始答题...</p>
      </div>
      <p v-else class="text-dim">答题进行中（作答界面即将开放）</p>
    </div>

    <p style="text-align: center; margin-top: 16px">
      <button class="btn btn-ghost" style="font-size: 13px; padding: 8px 16px" @click="exit">退出</button>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { QuizWS } from '../ws'
import { Ev } from '../ws/types'
import { useQuizStore } from '../stores/quiz'
import { userApi } from '../api/user'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()
const store = useQuizStore()
const quizId = Number(route.params.id)

let ws: QuizWS | null = null
let refreshTimer: number | null = null

const statusText = computed(() => {
  const m: Record<string, string> = {
    WAITING: '未开始',
    RUNNING: '进行中',
    PAUSED: '已暂停',
    RUSHING: '抢答中',
    ANSWERING: '答题中',
    REVEALING: '公布答案',
    FINISHED: '已结束',
  }
  return m[store.status] || store.status
})

onMounted(async () => {
  const token = userApi.quizToken(quizId)
  if (!token) {
    router.replace('/join')
    return
  }
  ws = new QuizWS({
    token,
    onStatus: (s) => (store.wsStatus = s),
    onEvent: (msg) => {
      switch (msg.event) {
        case Ev.Sync:
          store.applySync(msg.data)
          break
        case Ev.ActivityStart:
        case Ev.ActivityResume:
          store.status = 'RUNNING'
          if (msg.data) store.applySync(msg.data)
          break
        case Ev.ActivityPause:
          store.status = 'PAUSED'
          break
        case Ev.ActivityEnd:
          store.status = 'FINISHED'
          break
      }
    },
  })
  // 轮询兜底（WS 断开时仍能感知状态）
  refreshTimer = window.setInterval(async () => {
    if (store.wsStatus !== 'open') {
      try {
        const info = await userApi.quizInfo(quizId)
        if (info.quiz) {
          store.status = info.quiz.status
          if (store.quiz) store.quiz.participant_count = info.participant_count
        }
      } catch {
        /* token 失效时忽略 */
      }
    }
  }, 5000)
})

onUnmounted(() => {
  ws?.close()
  if (refreshTimer) clearInterval(refreshTimer)
})

function exit() {
  if (quizId) localStorage.removeItem(LS.userToken(quizId))
  router.replace('/join')
}
</script>

<style scoped>
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
</style>
