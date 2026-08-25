<template>
  <div class="page" style="max-width: 480px; padding-top: 10vh">
    <div class="card" style="padding: 32px; text-align: center">
      <template v-if="joining">
        <div class="pulse-dot"></div>
        <p style="margin-top: 14px">正在进入答题...</p>
      </template>

      <template v-else>
        <h1 style="font-size: 22px; margin-bottom: 6px">📝 加入答题</h1>
        <template v-if="brief">
          <p style="font-size: 16px; font-weight: 700; margin-bottom: 6px">{{ brief.title }}</p>
          <p class="text-dim" style="font-size: 13px; margin-bottom: 18px">{{ brief.description || '' }}</p>
          <div style="display: flex; justify-content: center; gap: 24px; margin-bottom: 22px">
            <div>
              <div style="font-size: 22px; font-weight: 800">{{ brief.participant_count }}</div>
              <div class="text-dim" style="font-size: 12px">已参与</div>
            </div>
            <div>
              <div style="font-size: 22px; font-weight: 800">{{ statusText }}</div>
              <div class="text-dim" style="font-size: 12px">状态</div>
            </div>
          </div>
        </template>
        <p v-if="err" style="color: var(--danger); margin-bottom: 14px; font-size: 14px">{{ err }}</p>

        <div style="display: flex; flex-direction: column; gap: 10px">
          <div v-if="quizIdInput" style="display: flex; gap: 8px">
            <input v-model="quizIdInput" class="input" type="number" placeholder="答题编号" min="1" style="flex: 1" />
            <button class="btn btn-primary" style="padding: 12px 18px" :disabled="!quizIdInput" @click="go(Number(quizIdInput))">
              进入
            </button>
          </div>
          <button v-else class="btn btn-ghost" @click="$router.push('/login')">切换账号（{{ nick }}）</button>
        </div>
      </template>
    </div>
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
const err = ref('')
const brief = ref<{ id: number; title: string; description: string; status: string; participant_count: number } | null>(null)
const quizIdInput = ref('')

const linkQuizId = computed(() => Number(route.params.id || 0))
const nick = computed(() => localStorage.getItem(LS.userNick) || '已登录')

const statusText = computed(() => {
  const m: Record<string, string> = {
    WAITING: '未开始',
    RUNNING: '进行中',
    ANSWERING: '答题中',
    PAUSED: '已暂停',
    RUSHING: '抢答中',
    REVEALING: '公布答案',
    FINISHED: '已结束',
  }
  return brief.value ? m[brief.value.status] || brief.value.status : '—'
})

onMounted(async () => {
  // 未登录 → 登录后回来
  if (!globalToken()) {
    router.replace({
      path: '/login',
      query: linkQuizId.value ? { redirect: `/join/${linkQuizId.value}` } : {},
    })
    return
  }
  if (linkQuizId.value > 0) {
    go(linkQuizId.value)
  } else {
    // 无链接：手填编号
    quizIdInput.value = ''
  }
})

async function go(quizId: number) {
  if (!quizId || quizId <= 0) return
  joining.value = true
  err.value = ''
  try {
    // 拉取活动信息（不存在则提示）
    brief.value = await userApi.quizBrief(quizId)
    // 已有答题 token 直接进
    if (localStorage.getItem(LS.userToken(quizId))) {
      router.replace(`/quiz/${quizId}`)
      return
    }
    // 加入换取答题 token
    const { token, quiz, user } = await userApi.joinQuiz(quizId)
    localStorage.setItem(LS.userToken(quiz.id), token)
    localStorage.setItem(LS.userId(quiz.id), String(user.id))
    localStorage.setItem(LS.nickname(quiz.id), user.nickname)
    router.replace(`/quiz/${quiz.id}`)
  } catch (e: any) {
    joining.value = false
    err.value = e?.response?.data?.msg || '加入失败，请检查答题编号'
  }
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
