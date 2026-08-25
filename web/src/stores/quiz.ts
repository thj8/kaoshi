import { defineStore } from 'pinia'
import type { SyncData } from '../ws/types'

/**
 * 用户端答题状态（服务端通过 WS 推送，本地只做展示）
 */

export interface QuizState extends SyncData {
  wsStatus: 'connecting' | 'open' | 'closed' | 'retrying'
  /** 本地渲染的剩余秒数（由页面 ticker 驱动） */
  remainMs: number
  /** 最近一次个人判分结果 */
  lastResult: any
  /** 是否已提交当前题 */
  submitted: boolean
  /** 抢答状态 */
  rushState: 'idle' | 'active' | 'won' | 'lost' | 'ended'
  rushRank: number
}

export const useQuizStore = defineStore('quiz', {
  state: (): QuizState => ({
    quiz: null,
    status: 'WAITING',
    question: null,
    deadline_at: 0,
    rush_active: false,
    me: null,
    server_now: 0,
    wsStatus: 'closed',
    remainMs: 0,
    lastResult: null,
    submitted: false,
    rushState: 'idle',
    rushRank: 0,
  }),
  getters: {
    answeredCount(state): number {
      return state.me?.answered ?? 0
    },
  },
  actions: {
    applySync(sync: SyncData) {
      this.quiz = sync.quiz
      this.status = sync.status
      this.question = sync.question
      this.deadline_at = sync.deadline_at
      this.rush_active = sync.rush_active
      this.me = sync.me
      this.server_now = sync.server_now
      this.submitted = false
      // 恢复抢答状态
      if (this.status === 'RUSHING') this.rushState = 'active'
      else if (this.rushState === 'active') this.rushState = 'ended'
      this.remainMs = this.deadline_at ? Math.max(0, this.deadline_at - Date.now()) : 0
    },
  },
})
