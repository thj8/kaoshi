<template>
  <div class="auth-wrap">
    <div class="auth-box">
      <div class="mark">答</div>
      <p class="eyebrow">实时理论竞赛</p>
      <h1>理论答题系统</h1>

      <form class="auth-card" @submit.prevent="submit">
        <div class="field">
          <label for="f-user">用户名</label>
          <input id="f-user" v-model="username" class="input" autocomplete="username" />
        </div>
        <div class="field">
          <label for="f-pass">密码</label>
          <input id="f-pass" v-model="password" class="input" type="password" autocomplete="current-password" />
        </div>
        <p v-if="err" class="err" role="alert">{{ err }}</p>
        <button class="btn btn-primary submit" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>

      <!-- 签名元素：答题卡涂卡气泡，登录 = 涂下第一格 -->
      <div class="bubbles" aria-hidden="true">
        <span class="bubble filled">A</span>
        <span class="bubble">B</span>
        <span class="bubble">C</span>
        <span class="bubble">D</span>
      </div>

      <p v-if="redirectHint" class="hint">登录后自动进入{{ redirectHint }}</p>
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
    radial-gradient(900px 460px at 50% -8%, rgba(0, 113, 227, 0.09), transparent 70%),
    var(--bg);
}
.auth-box {
  width: 352px;
  max-width: 100%;
  text-align: center;
}
.mark {
  width: 56px;
  height: 56px;
  margin: 0 auto 18px;
  border-radius: 16px;
  background: var(--primary);
  color: #fff;
  font-size: 25px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 10px 24px rgba(0, 113, 227, 0.26);
  animation: rise 0.45s ease both;
}
.eyebrow {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-dim);
  letter-spacing: 0.14em;
}
h1 {
  font-size: 27px;
  font-weight: 800;
  letter-spacing: -0.025em;
  margin-top: 4px;
}
.auth-card {
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 20px;
  box-shadow: var(--shadow);
  padding: 22px 20px 20px;
  margin-top: 28px;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: 14px;
  animation: rise 0.45s ease 0.05s both;
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
}
.submit {
  width: 100%;
  margin-top: 2px;
}

/* 签名：涂卡气泡 */
.bubbles {
  display: flex;
  justify-content: center;
  gap: 18px;
  margin-top: 26px;
  animation: rise 0.45s ease 0.1s both;
}
.bubble {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  border: 1.5px solid rgba(0, 0, 0, 0.18);
  color: var(--text-dim);
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}
.bubble.filled {
  border-color: var(--primary);
  background: var(--primary);
  color: #fff;
  box-shadow: 0 4px 12px rgba(0, 113, 227, 0.3);
}
.bubble:not(.filled) {
  animation: ghost 3s ease-in-out infinite;
}
.bubble:not(.filled):nth-child(2) { animation-delay: 0.5s; }
.bubble:not(.filled):nth-child(3) { animation-delay: 1s; }
.bubble:not(.filled):nth-child(4) { animation-delay: 1.5s; }
@keyframes ghost {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}

.hint {
  margin-top: 18px;
  color: var(--text-dim);
  font-size: 13px;
}
@keyframes rise {
  from { opacity: 0; transform: translateY(10px); }
}
@media (prefers-reduced-motion: reduce) {
  .mark,
  .auth-card,
  .bubbles,
  .bubble {
    animation: none;
  }
}
</style>
