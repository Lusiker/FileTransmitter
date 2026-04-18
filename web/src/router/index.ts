import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue')
    },
    {
      path: '/sender',
      name: 'Sender',
      component: () => import('@/views/Sender/index.vue')
    },
    {
      path: '/receiver',
      name: 'Receiver',
      component: () => import('@/views/Receiver/index.vue')
    },
    {
      path: '/admin',
      name: 'Admin',
      component: () => import('@/views/Admin.vue')
    }
  ]
})

export default router