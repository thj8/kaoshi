/** WS 协议类型定义（与 server/internal/ws/protocol.go 对齐） */

export interface QuizBrief {
  id: number
  title: string
  description: string
  mode: string
  show_answer: boolean
  show_analysis: boolean
  show_ranking: boolean
  participant_count: number
}

export interface Option {
  label: string
  content: string
}

export interface QuestionBrief {
  id: number
  index: number
  total: number
  type: 'single' | 'multiple' | 'judge'
  content: string
  options: Option[]
  score: number
  required: boolean
  time_limit: number
}

export interface MeInfo {
  user_id: number
  nickname: string
  score: number
  answered: number
}

export interface SyncData {
  quiz: QuizBrief | null
  status: string
  question: QuestionBrief | null
  deadline_at: number
  rush_active: boolean
  my_rush_rank: number // 0=未抢 -1=失败 >0=成功
  rush_winners: RushWinner[] | null
  me: MeInfo | null
  server_now: number
}

export interface RushWinner {
  user_id: number
  nickname: string
  rank: number
  bonus: number
}

export interface RushResultData {
  question_id: number
  rank: number
  nickname: string
  bonus: number
  score: number
  reason?: string
}

export interface RushEndData {
  question_id: number
  winners: RushWinner[]
  answer_deadline_at: number
  server_now: number
}

export interface AnswerResultData {
  question_id: number
  answer: string
  is_correct: boolean
  score: number
  total_score: number
  correct_answer?: string
  analysis?: string
  revealed: boolean
}

export interface RevealData {
  question_id: number
  correct_answer: string
  analysis?: string
  stats?: { total: number; correct: number; wrong: number }
  /** 用户端个人反馈（服务端随 answer:reveal 下发） */
  my_answer?: string // 本人提交的（"-"=未答）
  my_score?: number
  is_correct?: boolean
}

export interface RankingItem {
  rank: number
  user_id: number
  nickname: string
  score: number
  correct: number
}

export interface RankingData {
  items: RankingItem[]
}

export interface CountdownData {
  question_id: number
  remain_sec: number
  deadline_at: number
}

export interface WSMessage<T = any> {
  event: string
  data: T
  ts?: number
}

/** 服务端事件名 */
export const Ev = {
  Sync: 'sync',
  ActivityStart: 'activity:start',
  ActivityPause: 'activity:pause',
  ActivityResume: 'activity:resume',
  ActivityEnd: 'activity:end',
  QuestionPublish: 'question:publish',
  QuestionCountdown: 'question:countdown',
  AnswerAccepted: 'answer:accepted',
  AnswerResult: 'answer:result',
  AnswerReveal: 'answer:reveal',
  RushStart: 'rush:start',
  RushSuccess: 'rush:success',
  RushFailed: 'rush:failed',
  RushEnd: 'rush:end',
  RankingUpdate: 'ranking:update',
  StatisticsUpdate: 'statistics:update',
  Error: 'error',
} as const
