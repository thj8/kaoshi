import { createRouter, createWebHistory } from 'vue-router'
import { LS } from '../api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 用户端
    { path: '/', redirect: '/join' },
    { path: '/join', component: () => import('../user/JoinPage.vue') },
    { path: '/join/:id', component: () => import('../user/JoinPage.vue') },
    { path: '/quiz/:id', component: () => import('../user/QuizPage.vue') },
    // 管理端
    { path: '/admin/login', component: () => import('../admin/LoginPage.vue') },
    {
      path: '/admin',
      component: () => import('../admin/AdminLayout.vue'),
      children: [
        { path: '', component: () => import('../admin/QuizListPage.vue') },
        { path: 'users', component: () => import('../admin/UserManagePage.vue') },
        { path: 'quiz/:id', component: () => import('../admin/QuizEditPage.vue') },
        { path: 'quiz/:id/console', component: () => import('../admin/ConsolePage.vue') },
      ],
    },
  ],
})

// 管理端路由守卫：未登录跳登录页
router.beforeEach((to) => {
  if (to.path.startsWith('/admin') && to.path !== '/admin/login') {
    if (!localStorage.getItem(LS.adminToken)) {
      return '/admin/login'
    }
  }
})

export default router
