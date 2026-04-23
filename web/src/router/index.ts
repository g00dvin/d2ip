import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'dashboard', component: () => import('@/views/DashboardView.vue') },
  { path: '/pipeline', name: 'pipeline', component: () => import('@/views/PipelineView.vue') },
  { path: '/config', name: 'config', component: () => import('@/views/ConfigView.vue') },
  { path: '/categories', name: 'categories', component: () => import('@/views/CategoriesView.vue') },
  { path: '/cache', name: 'cache', component: () => import('@/views/CacheView.vue') },
  { path: '/source', name: 'source', component: () => import('@/views/SourceView.vue') },
  { path: '/routing', name: 'routing', component: () => import('@/views/RoutingView.vue') },
]

export default createRouter({
  history: createWebHashHistory(),
  routes,
})