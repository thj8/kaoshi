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
