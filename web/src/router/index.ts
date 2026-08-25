import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // 用户端
    { path: '/', redirect: '/join' },
    { path: '/join', component: () => import('../user/JoinPage.vue') },
    // 管理端
    { path: '/admin/login', component: () => import('../admin/LoginPage.vue') },
    { path: '/admin', component: () => import('../admin/AdminLayout.vue') },
  ],
})

export default router
