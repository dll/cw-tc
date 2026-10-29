import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import Login from '@/pages/Login.vue'
import ChangePassword from '@/pages/ChangePassword.vue'
import Orders from '@/pages/Orders.vue'
import Refunds from '@/pages/Refunds.vue'
import Certs from '@/pages/Certs.vue'
import Users from '@/pages/Users.vue'
import Broadcast from '@/pages/Broadcast.vue'
import Staff from '@/pages/Staff.vue'
import Courses from '@/pages/Courses.vue'
import ContentRequests from '@/pages/ContentRequests.vue'
import Operations from '@/pages/Operations.vue'
import Majors from '@/pages/Majors.vue'
import TeacherCourses from '@/pages/TeacherCourses.vue'
import TeacherGrade from '@/pages/TeacherGrade.vue'
import Audit from '@/pages/Audit.vue'
import Placeholder from '@/pages/Placeholder.vue'
import { hasToken } from '@/api/client'

/// 路由表按 PRD v2 §8.1 的站点地图固化。
/// 权限由后端 RBAC + 资源归属校验把关（前端只做菜单可见性，不作为安全边界）。
const routes: RouteRecordRaw[] = [
  { path: '/login', component: Login },
  { path: '/change-password', component: ChangePassword },
  { path: '/', redirect: '/admin/courses' },

  // —— 管理端（角色：管理员）——
  { path: '/admin/courses', component: Courses },
  { path: '/admin/content-requests', component: ContentRequests },
  { path: '/admin/operations', component: Operations },
  { path: '/admin/majors', component: Majors },
  { path: '/admin/orders', component: Orders },
  { path: '/admin/refunds', component: Refunds },
  { path: '/admin/certs', component: Certs },
  { path: '/admin/kb', component: Placeholder, props: { title: '知识库重建', detail: '切片数与向量数须为真实统计' } },
  { path: '/admin/broadcast', component: Broadcast },
  { path: '/admin/users', component: Users },
  { path: '/admin/staff', component: Staff },
  { path: '/admin/audit', component: Audit },

  // —— 教师工作台（角色：教师）——
  { path: '/teacher/courses', component: TeacherCourses },
  { path: '/teacher/courses/new', component: Placeholder, props: { title: '新建课程', detail: '提交后需管理员审核上架' } },
  { path: '/teacher/review', component: TeacherGrade },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.path !== '/login' && !hasToken()) return '/login'
  if (to.path === '/login' && hasToken()) return '/admin/courses'
  return true
})
