import Vue from 'vue'
import Router from 'vue-router'

import AppLayout from '@/layout/AppLayout.vue'
import HomeView from '@/layout/HomeView.vue'
import store from '@/store'
import Forbidden from '@/views/Forbidden.vue'
import Login from '@/views/Login.vue'
import NotFound from '@/views/NotFound.vue'

import { installRouterGuards } from './guards'

Vue.use(Router)

export function createRouter(storeInstance = store, options = {}) {
  const router = new Router({
    mode: options.mode || 'history',
    routes: [
      {
        path: '/login',
        name: 'login',
        component: Login,
      },
      {
        path: '/403',
        name: 'forbidden',
        component: Forbidden,
        meta: { requiresAuth: true },
      },
      {
        path: '/',
        component: AppLayout,
        meta: { requiresAuth: true },
        children: [
          {
            path: '',
            name: 'home',
            component: HomeView,
          },
        ],
      },
      {
        path: '*',
        name: 'not-found',
        component: NotFound,
        meta: { requiresAuth: true },
      },
    ],
  })

  installRouterGuards(router, storeInstance)
  return router
}

const router = createRouter()

export default router
