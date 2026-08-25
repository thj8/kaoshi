<template>
  <div class="page" style="max-width: 440px; padding-top: 9vh">
    <div class="card" style="padding: 32px">
      <h1 style="font-size: 22px; margin-bottom: 4px">答题系统</h1>
      <p class="text-dim" style="margin-bottom: 22px">账号由管理员创建，凭用户名密码登录</p>


      <form @submit.prevent="submit">
        <div style="margin-bottom: 14px">
          <input v-model="username" class="input" placeholder="用户名" autocomplete="username" />
        </div>
        <div style="margin-bottom: 14px">
          <input v-model="password" class="input" type="password" placeholder="密码" autocomplete="current-password" />
        </div>
        <p v-if="err" style="color: var(--danger); margin-bottom: 12px; font-size: 14px">{{ err }}</p>
        <button class="btn btn-primary" style="width: 100%" :disabled="loading">
          {{ loading ? '请稍候...' : '登 录' }}
        </button>
      </form>

      <p v-if="redirectHint" class="text-dim" style="margin-top: 14px; font-size: 13px; text-align: center">
        登录后将自动进入：{{ redirectHint }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { userApi, globalToken } from '../api/user'
import { LS } from '../api'

const route = useRoute()
const router = useRouter()

const username = ref('')
const password = ref('')
const err = ref('')
const loading = ref(false)

const redirectHint = computed(() => {
  const r = (route.query.redirect as string) || ''
  const m = r.match(/\/join\/(\d+)/)
  if (m) {
    const id = m[1]
    return `答题 #${id}`
  }
  return ''
})

async function submit() {
  if (!username.value.trim() || !password.value) {
    err.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  err.value = ''
  try {
    const r = await userApi.login(username.value.trim(), password.value)
    localStorage.setItem(LS.userGlobalToken, r.token)
    localStorage.setItem(LS.userNick, r.user.nickname)
    const redirect = (route.query.redirect as string) || ''
    if (redirect) {
      router.replace(redirect)
    } else {
      router.replace('/join')
    }
  } catch (e: any) {
    err.value = e?.response?.data?.msg || '操作失败'
  } finally {
    loading.value = false
  }
}

// 已登录直接跳走
if (globalToken()) {
  const redirect = (route.query.redirect as string) || '/join'
  router.replace(redirect)
}
</script>
