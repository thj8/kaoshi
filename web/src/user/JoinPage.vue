<template>
  <div class="page" style="max-width: 480px; padding-top: 10vh">
    <div class="card" style="padding: 32px">
      <h1 style="font-size: 22px; margin-bottom: 4px">📝 加入答题</h1>
      <p class="text-dim" style="margin-bottom: 24px">输入昵称，实时作答</p>
      <form @submit.prevent="doJoin">
        <div style="margin-bottom: 14px">
          <input v-model="nickname" class="input" placeholder="你的昵称" maxlength="32" />
        </div>
        <div v-if="quizId <= 0" style="margin-bottom: 20px">
          <input
            v-model="quizIdInput"
            class="input"
            type="number"
            placeholder="答题编号（来自管理员分享的链接）"
            min="1"
          />
        </div>
        <div v-else-if="quizTitle" style="margin-bottom: 20px; font-size: 14px; color: var(--text-dim)">
          答题：<b style="color: var(--text)">{{ quizTitle }}</b>
        </div>
        <p v-if="err" style="color: var(--danger); margin-bottom: 12px; font-size: 14px">{{ err }}</p>
        <button class="btn btn-primary" style="width: 100%" :disabled="loading">
          {{ loading ? '进入中...' : '进入答题' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userApi } from '../api/user'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()

const nickname = ref('')
const quizIdInput = ref('')
const err = ref('')
const loading = ref(false)
const quizTitle = ref('')

const quizId = computed(() => Number(route.params.id || 0))

onMounted(() => {
  // 从链接进入：显示答题名称；已加入过则直接进入
  if (quizId.value > 0) {
    if (localStorage.getItem(LS.userToken(quizId.value))) {
      router.replace(`/quiz/${quizId.value}`)
      return
    }
    // 公开信息（无需登录）
    fetch(`/api/quiz/${quizId.value}/brief`)
      .then((r) => r.json())
      .then((d) => {
        if (d.code === 0) quizTitle.value = d.data?.title || ''
      })
      .catch(() => {})
  }
})

async function doJoin() {
  const qid = quizId.value > 0 ? quizId.value : Number(quizIdInput.value)
  if (!nickname.value.trim()) {
    err.value = '请输入昵称'
    return
  }
  if (!qid || qid <= 0) {
    err.value = '请输入答题编号'
    return
  }
  loading.value = true
  err.value = ''
  try {
    const { token, quiz, user } = await userApi.join(nickname.value.trim(), qid)
    localStorage.setItem(LS.userToken(quiz.id), token)
    localStorage.setItem(LS.userId(quiz.id), String(user.id))
    localStorage.setItem(LS.nickname(quiz.id), user.nickname)
    router.push(`/quiz/${quiz.id}`)
  } catch (e: any) {
    err.value = e?.response?.data?.msg || '加入失败，请检查答题编号'
  } finally {
    loading.value = false
  }
}
</script>
