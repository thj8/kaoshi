<template>
  <div class="auth-wrap">
    <div class="auth-box">
      <div class="mark">答</div>
      <h1>理论答题系统</h1>
      <p class="sub">使用管理员分配的账号登录</p>

      <form @submit.prevent="submit">
        <div class="field">
          <label>用户名</label>
          <input v-model="username" class="input" placeholder="请输入用户名" autocomplete="username" />
        </div>
        <div class="field">
          <label>密码</label>
          <input v-model="password" class="input" type="password" placeholder="请输入密码" autocomplete="current-password" />
        </div>
        <p v-if="err" class="err">{{ err }}</p>
        <button class="btn btn-primary submit" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>

      <p v-if="redirectHint" class="hint">登录后将自动进入{{ redirectHint }}</p>
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

<style scoped>
.auth-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background:
    radial-gradient(1000px 500px at 50% -10%, rgba(0, 113, 227, 0.08), transparent 70%),
    var(--bg);
}
.auth-box {
  width: 360px;
  max-width: 100%;
  text-align: center;
}
.mark {
  width: 72px;
  height: 72px;
  margin: 0 auto 20px;
  border-radius: 20px;
  background: var(--primary);
  color: #fff;
  font-size: 32px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 12px 28px rgba(0, 113, 227, 0.28);
}
h1 {
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.sub {
  margin: 8px 0 30px;
  color: var(--text-dim);
  font-size: 15px;
}
.field {
  text-align: left;
  margin-bottom: 16px;
}
.field label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-dim);
  margin-bottom: 6px;
}
.err {
  color: var(--danger);
  font-size: 14px;
  margin-bottom: 12px;
}
.submit {
  width: 100%;
  margin-top: 6px;
}
.hint {
  margin-top: 18px;
  color: var(--text-dim);
  font-size: 13px;
}
</style>
