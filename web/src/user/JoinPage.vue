<template>
  <div class="page" style="max-width: 520px; padding-top: 8vh">
    <div class="card" style="padding: 28px">
      <template v-if="joining">
        <div class="pulse-dot"></div>
        <p style="margin-top: 14px; text-align: center">正在进入答题...</p>
      </template>

      <template v-else>
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px">
          <h1 style="font-size: 20px">📝 加入答题</h1>
          <button class="btn btn-ghost" style="padding: 6px 12px; font-size: 13px" @click="$router.push('/login')">
            切换账号（{{ nick }}）
          </button>
        </div>

        <p v-if="err" style="color: var(--danger); margin-bottom: 14px; font-size: 14px">{{ err }}</p>

        <template v-if="linkBrief">
          <!-- 深链 /join/<id>：展示指定活动确认进入 -->
          <p style="font-size: 16px; font-weight: 700; margin-bottom: 4px">{{ linkBrief.title }}</p>
          <p class="text-dim" style="font-size: 13px; margin-bottom: 18px">{{ linkBrief.description || '' }}</p>
        </template>

        <div v-if="linkBrief" style="display: flex; flex-direction: column; gap: 10px">
          <button class="btn btn-primary" @click="go(linkBrief.id)">进入答题</button>
        </div>

        <template v-else>
          <!-- 活动列表（可加入 = WAITING） -->
          <p v-if="loadingList" class="text-dim" style="font-size: 14px">加载中...</p>
          <template v-else-if="quizzes.length">
            <div
              v-for="q in quizzes"
              :key="q.id"
              class="quiz-item"
              @click="go(q.id)"
            >
              <div>
                <div style="font-weight: 700; font-size: 15px">{{ q.title }}</div>
                <div class="text-dim" style="font-size: 12px; margin-top: 4px">
                  #{{ q.id }} · {{ q.mode === 'rush' ? '抢答模式' : '普通模式' }} · {{ q.participant_count }} 人已加入
                </div>
              </div>
              <span class="enter-arrow">进入 →</span>
            </div>
          </template>
          <p v-else class="text-dim" style="font-size: 14px; text-align: center; padding: 18px 0">
            暂无可加入的答题活动，请等待管理员创建并开启
          </p>
        </template>
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
const loadingList = ref(true)
const err = ref('')
const quizzes = ref<{ id: number; title: string; description: string; mode: string; participant_count: number }[]>([])
const linkBrief = ref<{ id: number; title: string; description: string } | null>(null)
const nick = computed(() => localStorage.getItem(LS.userNick) || '已登录')

const linkQuizId = computed(() => Number(route.params.id || 0))

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
    // 深链：展示该活动并自动进入
    try {
      linkBrief.value = await userApi.quizBrief(linkQuizId.value)
      go(linkQuizId.value)
    } catch {
      err.value = '答题活动不存在'
    }
  } else {
    // 活动列表
    try {
      const r = await userApi.quizList()
      quizzes.value = r.items
    } catch {
      err.value = '活动列表加载失败，请刷新重试'
    } finally {
      loadingList.value = false
    }
  }
})

async function go(quizId: number) {
  if (!quizId || quizId <= 0) return
  joining.value = true
  err.value = ''
  try {
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
    err.value = e?.response?.data?.msg || '加入失败'
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
.quiz-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
  border-radius: 12px;
  background: var(--bg-soft, rgba(255, 255, 255, 0.04));
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
}
.quiz-item:hover {
  border-color: var(--primary);
  transform: translateY(-1px);
}
.quiz-item + .quiz-item {
  margin-top: 10px;
}
.enter-arrow {
  font-size: 13px;
  color: var(--primary);
  white-space: nowrap;
}
</style>
