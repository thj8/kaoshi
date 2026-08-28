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
    return unwrap<{ id: number; code: string; title: string; description: string; status: string; participant_count: number }>(
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
}

function quizIdFromPath(): string {
  const m = location.pathname.match(/\/quiz\/([0-9a-z]+)/)
  return m ? m[1] : ''
}
