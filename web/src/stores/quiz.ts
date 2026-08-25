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
  /** 抢答状态：idle=等待抢答开始 active=可抢 won=成功 lost=失败 ended=本题结束 */
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
    my_rush_rank: 0,
    rush_winners: null,
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
    /** 当前题是否抢答题且我已获答 */
    iAmWinner(state): boolean {
      return state.my_rush_rank > 0
    },
  },
  actions: {
    applySync(sync: SyncData) {
      this.quiz = sync.quiz
      this.status = sync.status
      this.question = sync.question
      this.deadline_at = sync.deadline_at
      this.rush_active = sync.rush_active
      this.my_rush_rank = sync.my_rush_rank || 0
      this.rush_winners = sync.rush_winners
      this.me = sync.me
      this.server_now = sync.server_now
      this.submitted = false
      // 恢复抢答状态
      if (sync.status === 'RUSHING') {
        this.rushState = sync.my_rush_rank > 0 ? 'won' : sync.my_rush_rank < 0 ? 'lost' : 'active'
      } else if (sync.my_rush_rank > 0) {
        this.rushState = 'won'
      } else if (sync.rush_winners && sync.rush_winners.length > 0) {
        this.rushState = 'ended'
      } else {
        this.rushState = 'idle'
      }
      this.rushRank = sync.my_rush_rank || 0
      this.remainMs = this.deadline_at ? Math.max(0, this.deadline_at - Date.now()) : 0
    },
  },
})
