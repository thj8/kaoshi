import { http, unwrap, LS } from './index'
import type { AnswerResultData, RankingData } from '../ws/types'

export interface QuizInfo {
  quiz: {
    id: number
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

export const userApi = {
  async join(nickname: string, quizId: number) {
    return unwrap<JoinResp>(
      await http.post('/api/join', { nickname, quiz_id: quizId })
    )
  },
  async quizInfo(quizId: number) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<QuizInfo>(
      await http.get(`/api/quiz/${quizId}`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  quizToken(quizId: number) {
    return localStorage.getItem(LS.userToken(quizId)) || ''
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
  async ranking(quizId: number) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<{ visible: boolean; items?: RankingData['items']; mine_rank: number }>(
      await http.get(`/api/quiz/${quizId}/ranking`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
  async result(quizId: number) {
    const token = localStorage.getItem(LS.userToken(quizId)) || ''
    return unwrap<Record<string, any>>(
      await http.get(`/api/quiz/${quizId}/result`, { headers: { Authorization: `Bearer ${token}` } })
    )
  },
}

function quizIdFromPath(): number {
  const m = location.pathname.match(/\/quiz\/(\d+)/)
  return m ? Number(m[1]) : 0
}
