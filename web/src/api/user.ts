import { http, unwrap, LS } from './index'
import type { AnswerResultData, RankingData, RushResultData } from '../ws/types'

export interface QuizInfo {
  quiz: {
    id: number
    code: string
    title: string
    description: string
    status: string
    mode: string
    show_answer: boolean
    show_analysis: boolean
    show_ranking: boolean
  }
  participant_count: number
  me: { user_id: number; nickname: string }
}

export interface JoinResp {
  token: string
  quiz: QuizInfo['quiz']
  user: { id: number; nickname: string }
}

export interface AuthUser {
  id: number
  username: string
  nickname: string
}

/** 全局用户 token（登录态） */
export function globalToken(): string {
  return localStorage.getItem(LS.userGlobalToken) || ''
}

export interface PaperQuestion {
  id: number
  index: number
  type: 'single' | 'multiple' | 'judge'
  content: string
  score: number
  options: { label: string; content: string }[]
  my_answer: string | null
}

export interface Paper {
  title: string
  mode: string
  status: string
  total: number
  question_count: number // 真实题数（WAITING 时 total=0 防提前看题，仅下发数量）
  deadline_at: number
  server_now: number
  submitted: boolean
  score: number
  questions: PaperQuestion[]
}

export interface PaperSummary {
  score: number
  answered: number
  total: number
  correct: number
  wrong: number
  rank: number
  finished: boolean
}

export const userApi = {
  async quizList(): Promise<{ items: { id: number; code: string; title: string; description: string; mode: string; participant_count: number; joined: boolean }[] }> {
    return unwrap(await http.get('/api/quizzes', { headers: { Authorization: `Bearer ${globalToken()}` } }))
  },
  /** 我参加过的全部比赛（含已结束） */
  async myQuizzes(): Promise<{ items: { quiz_id: number; code: string; title: string; status: string; mode: string; score: number; correct: number; wrong: number; joined_at: string; participant_count: number }[] }> {
    return unwrap(await http.get('/api/my/quizzes', { headers: { Authorization: `Bearer ${globalToken()}` } }))
  },
  async login(username: string, password: string) {
    return unwrap<{ token: string; user: AuthUser }>(
      await http.post('/api/auth/login', { username, password })
    )
  },
  async me() {
    return unwrap<AuthUser>(
      await http.get('/api/auth/me', { headers: { Authorization: `Bearer ${globalToken()}` } })
    )
  },
  /** 已登录用户加入答题，换取答题作用域 token */
  async joinQuiz(quizId: string) {
    return unwrap<JoinResp>(
      await http.post('/api/join', { quiz_id: quizId }, { headers: { Authorization: `Bearer ${globalToken()}` } })
    )
  },
  async quizBrief(quizId: string) {
    return unwrap<{ id: number; code: string; title: string; description: string; status: string; mode: string; participant_count: number }>(
      await http.get(`/api/quiz/${quizId}/brief`)
    )
  },
  async quizInfo(quizId: string) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<QuizInfo>(
      await http.get(`/api/quiz/${quizId}`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  quizToken(quizId: string) {
    return localStorage.getItem(LS.userToken(quizId)) || ''
  },
  async rush(questionId: number) {
    const quizId = quizIdFromPath()
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<RushResultData>(
      await http.post(
        `/api/question/${questionId}/rush`,
        {},
        { headers: { Authorization: `Bearer ${token}` } }
      )
    )
  },
  async submitAnswer(questionId: number, answer: string, durationMs: number) {
    const quizId = quizIdFromPath()
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<AnswerResultData>(
      await http.post(
        `/api/question/${questionId}/answer`,
        { answer, duration: durationMs },
        { headers: { Authorization: `Bearer ${token}` } }
      )
    )
  },
  async ranking(quizId: string) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<{ visible: boolean; items?: RankingData['items']; mine_rank: number }>(
      await http.get(`/api/quiz/${quizId}/ranking`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  async result(quizId: string) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<Record<string, any>>(
      await http.get(`/api/quiz/${quizId}/result`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  /** ===== 考试（自由切题）模式 ===== */
  async paper(quizId: string) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<Paper>(
      await http.get(`/api/quiz/${quizId}/paper`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  /** 选择即保存（可反复修改，直到交卷/到时） */
  async savePaperAnswer(quizId: string, questionId: number, answer: string, durationMs = 0) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<{ answered: number; total: number }>(
      await http.post(
        `/api/quiz/${quizId}/paper/answer`,
        { question_id: questionId, answer, duration: durationMs },
        { headers: { Authorization: `Bearer ${token}` } }
      )
    )
  },
  async submitPaper(quizId: string) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<PaperSummary>(
      await http.post(`/api/quiz/${quizId}/paper/submit`, {}, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
}

function quizIdFromPath(): string {
  const m = location.pathname.match(/\/(quiz|exam)\/([0-9a-z]+)/)
  return m ? m[2] : ''
}
