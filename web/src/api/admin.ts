import { http, unwrap, authHeaders, LS } from './index'

export interface Option {
  label: string
  content: string
}

export interface Question {
  id: number
  quiz_id: number
  type: 'single' | 'multiple' | 'judge'
  content: string
  answer: string
  analysis: string
  score: number
  required: boolean
  sort: number
  time_limit: number
  options: Option[]
}

export interface Quiz {
  id: number
  title: string
  description: string
  status: string
  mode: 'normal' | 'rush'
  total_time: number
  per_question_time: number
  rush_enabled: boolean
  show_answer: boolean
  show_analysis: boolean
  show_ranking: boolean
  rush_winner_count: number
  rush_time: number
  rush_answer_time: number
  rush_bonus_score: number
  rush_wrong_score: number
  created_at: string
  started_at?: string
  ended_at?: string
}

export function adminToken(): string {
  return localStorage.getItem(LS.adminToken) || ''
}

function h() {
  return { headers: authHeaders(adminToken()) }
}

export const adminApi = {
  async login(username: string, password: string) {
    const r = await http.post<{ code: number; data: { token: string } }>('/api/admin/login', { username, password })
    return unwrap<{ token: string }>(r as never)
  },
  async listQuizzes() {
    return unwrap<Quiz[]>(await http.get('/api/admin/quizzes', h()))
  },
  async createQuiz(data: Partial<Quiz>) {
    return unwrap<Quiz>(await http.post('/api/admin/quiz', data, h()))
  },
  async updateQuiz(id: number, data: Partial<Quiz>) {
    return unwrap<Quiz>(await http.put(`/api/admin/quiz/${id}`, data, h()))
  },
  async deleteQuiz(id: number) {
    return unwrap<null>(await http.delete(`/api/admin/quiz/${id}`, h()))
  },
  async getQuiz(id: number) {
    return unwrap<{ quiz: Quiz; questions: Question[] }>(await http.get(`/api/admin/quiz/${id}`, h()))
  },
  async listQuestions(quizId: number) {
    return unwrap<Question[]>(await http.get(`/api/admin/quiz/${quizId}/questions`, h()))
  },
  async createQuestion(quizId: number, data: Partial<Question>) {
    return unwrap<Question>(await http.post(`/api/admin/quiz/${quizId}/questions`, data, h()))
  },
  async updateQuestion(qid: number, data: Partial<Question>) {
    return unwrap<Question>(await http.put(`/api/admin/question/${qid}`, data, h()))
  },
  async deleteQuestion(qid: number) {
    return unwrap<null>(await http.delete(`/api/admin/question/${qid}`, h()))
  },
}
