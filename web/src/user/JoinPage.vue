<template>
  <div class="page" style="max-width: 480px; padding-top: 10vh">
    <div class="card" style="padding: 32px">
      <h1 style="font-size: 22px; margin-bottom: 4px">📝 加入答题</h1>
      <p class="text-dim" style="margin-bottom: 24px">输入昵称与邀请码，实时作答</p>
      <form @submit.prevent="doJoin">
        <div style="margin-bottom: 14px">
          <input v-model="nickname" class="input" placeholder="你的昵称" maxlength="32" />
        </div>
        <div style="margin-bottom: 20px">
          <input
            v-model="inviteCode"
            class="input"
            placeholder="邀请码（6位）"
            maxlength="6"
            style="letter-spacing: 4px; text-transform: uppercase; text-align: center; font-size: 18px"
          />
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
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { userApi } from '../api/user'
import { LS } from '../api'

const router = useRouter()
const nickname = ref('')
const inviteCode = ref('')
const err = ref('')
const loading = ref(false)

async function doJoin() {
  if (!nickname.value.trim()) {
    err.value = '请输入昵称'
    return
  }
  if (inviteCode.value.trim().length !== 6) {
    err.value = '请输入 6 位邀请码'
    return
  }
  loading.value = true
  err.value = ''
  try {
    const { token, quiz, user } = await userApi.join(nickname.value.trim(), inviteCode.value.trim())
    localStorage.setItem(LS.userToken(quiz.id), token)
    localStorage.setItem(LS.userId(quiz.id), String(user.id))
    localStorage.setItem(LS.nickname(quiz.id), user.nickname)
    router.push(`/quiz/${quiz.id}`)
  } catch (e: any) {
    err.value = e?.response?.data?.msg || '加入失败，请检查邀请码'
  } finally {
    loading.value = false
  }
}
</script>
