<template>
  <div class="page" style="max-width: 420px; padding-top: 10vh">
    <div class="card" style="padding: 32px">
      <h1 style="font-size: 22px; margin-bottom: 4px">答题管理端</h1>
      <p class="text-dim" style="margin-bottom: 24px">实时答题系统 · 管理控制台</p>
      <form @submit.prevent="doLogin">
        <div style="margin-bottom: 14px">
          <input v-model="username" class="input" placeholder="用户名" autocomplete="username" />
        </div>
        <div style="margin-bottom: 20px">
          <input v-model="password" class="input" type="password" placeholder="密码" autocomplete="current-password" />
        </div>
        <p v-if="err" style="color: var(--danger); margin-bottom: 12px; font-size: 14px">{{ err }}</p>
        <button class="btn btn-primary" style="width: 100%" :disabled="loading">
          {{ loading ? '登录中...' : '登 录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../api/admin'
import { LS } from '../api'

const router = useRouter()
const username = ref('admin')
const password = ref('')
const err = ref('')
const loading = ref(false)

async function doLogin() {
  if (!username.value || !password.value) {
    err.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  err.value = ''
  try {
    const { token } = await adminApi.login(username.value, password.value)
    localStorage.setItem(LS.adminToken, token)
    router.push('/admin')
  } catch (e: any) {
    err.value = e?.response?.data?.msg || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>
