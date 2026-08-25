import { http, unwrap, LS } from './index'

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
  async join(nickname: string, inviteCode: string) {
    return unwrap<JoinResp>(
      await http.post('/api/join', { nickname, invite_code: inviteCode.trim().toUpperCase() })
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
}
