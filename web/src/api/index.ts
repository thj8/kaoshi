import axios from 'axios'

/**
 * REST 客户端：
 * - 开发环境走 vite proxy（/api -> localhost:18080）
 * - 生产构建可注入 VITE_API_BASE
 */
export const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  timeout: 10000,
})

// 业务错误（HTTP 200 + code!=0）统一转为异常，形状对齐 axios 错误，
// 各处 catch 里 e.response.data.msg 直接可用
http.interceptors.response.use((r) => {
  if (r.data && typeof r.data.code === 'number' && r.data.code !== 0) {
    const err: any = new Error(r.data.msg || '请求失败')
    err.response = { data: r.data }
    return Promise.reject(err)
  }
  return r
})

/** 统一响应体 { code, msg, data } */
export interface Resp<T = unknown> {
  code: number
  msg: string
  data: T
}

export function unwrap<T>(r: { data: Resp<T> }): T {
  return r.data.data
}

/** 本地存储 key 常量 */
export const LS = {
  adminToken: 'kaoshi_admin_token',
  /** 全局登录态（用户名/密码登录后） */
  userGlobalToken: 'kaoshi_user_token',
  userNick: 'kaoshi_user_nick',
  userToken: (quizId: number | string) => `kaoshi_token_${quizId}`,
  userId: (quizId: number | string) => `kaoshi_uid_${quizId}`,
  nickname: (quizId: number | string) => `kaoshi_nick_${quizId}`,
}

export function authHeaders(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}
