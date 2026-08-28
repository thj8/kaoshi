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
  code: string
  title: string
  description: string
  status: string
  mode: 'normal' | 'rush' | 'exam'
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
  req_score_single: number
  req_score_multiple: number
  req_score_judge: number
  rush_score_single: number
  rush_score_multiple: number
  rush_score_judge: number
  rush_deduct_single: number
  rush_deduct_multiple: number
  rush_deduct_judge: number
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
  async updateQuiz(id: string, data: Partial<Quiz>) {
    return unwrap<Quiz>(await http.put(`/api/admin/quiz/${id}`, data, h()))
  },
  async deleteQuiz(id: string) {
    return unwrap<null>(await http.delete(`/api/admin/quiz/${id}`, h()))
  },
  async getQuiz(id: string) {
    return unwrap<{ quiz: Quiz; questions: Question[] }>(await http.get(`/api/admin/quiz/${id}`, h()))
  },
  async listQuestions(id: string) {
    return unwrap<Question[]>(await http.get(`/api/admin/quiz/${id}/questions`, h()))
  },
  async createQuestion(quizId: string, data: Partial<Question>) {
    return unwrap<Question>(await http.post(`/api/admin/quiz/${quizId}/questions`, data, h()))
  },
  async updateQuestion(qid: number, data: Partial<Question>) {
    return unwrap<Question>(await http.put(`/api/admin/question/${qid}`, data, h()))
  },
  async deleteQuestion(qid: number) {
    return unwrap<null>(await http.delete(`/api/admin/question/${qid}`, h()))
  },
  async statistics(id: string) {
    return unwrap<Statistics>(await http.get(`/api/admin/quiz/${id}/statistics`, h()))
  },
  async listUsers(keyword = '') {
    return unwrap<(AdminUser & { quiz_count?: number })[]>(await http.get(`/api/admin/users?keyword=${encodeURIComponent(keyword)}`, h()))
  },
  async listInvitees(id: string) {
    return unwrap<{ items: Invitee[] }>(await http.get(`/api/admin/quiz/${id}/invitees`, h()))
  },
  async setInvitees(quizId: string, userIds: number[]) {
    return unwrap<null>(await http.put(`/api/admin/quiz/${quizId}/invitees`, { user_ids: userIds }, h()))
  },
}

export interface AdminUser {
  id: number
  username: string
  nickname: string
  created_at?: string
}

export interface Invitee {
  user_id: number
  username: string
  nickname: string
}

export interface Statistics {
  status: string
  participants: number
  finished: number
  avg_score: number
  max_score: number
  min_score: number
  avg_correct_rate: number
  questions: { index: number; question_id: number; type: string; content: string; answered: number; correct: number; wrong: number; correct_rate: number; avg_duration: number }[]
  ranking: { rank: number; user_id: number; nickname: string; score: number; correct: number; wrong: number; submitted_at: number }[]
}
