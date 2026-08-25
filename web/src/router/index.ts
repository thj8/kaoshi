import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 用户端
    { path: '/', redirect: '/join' },
    { path: '/join', component: () => import('../user/JoinPage.vue') },
    { path: '/quiz/:id', component: () => import('../user/QuizPage.vue') },
    // 管理端
    { path: '/admin/login', component: () => import('../admin/LoginPage.vue') },
    { path: '/admin', component: () => import('../admin/QuizListPage.vue') },
    { path: '/admin/quiz/:id', component: () => import('../admin/QuizEditPage.vue') },
    { path: '/admin/quiz/:id/console', component: () => import('../admin/ConsolePage.vue') },
  ],
})

export default router
