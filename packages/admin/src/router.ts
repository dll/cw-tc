import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import Login from '@/pages/Login.vue'
import ChangePassword from '@/pages/ChangePassword.vue'
import Orders from '@/pages/Orders.vue'
import Refunds from '@/pages/Refunds.vue'
import Certs from '@/pages/Certs.vue'
import Users from '@/pages/Users.vue'
import Broadcast from '@/pages/Broadcast.vue'
import Staff from '@/pages/Staff.vue'
import Admins from '@/pages/Admins.vue'
import Courses from '@/pages/Courses.vue'
import Comments from '@/pages/Comments.vue'
import Suggestions from '@/pages/Suggestions.vue'
import ContentRequests from '@/pages/ContentRequests.vue'
import Operations from '@/pages/Operations.vue'
import Statistics from '@/pages/Insights.vue'
import Entitlements from '@/pages/Entitlements.vue'
import Appeals from '@/pages/Appeals.vue'
import Deletions from '@/pages/Deletions.vue'
import Reconcile from '@/pages/Reconcile.vue'
import Knowledge from '@/pages/Knowledge.vue'
import TeacherAIDrafts from '@/pages/TeacherAIDrafts.vue'
import TeacherInsights from '@/pages/TeacherInsights.vue'
import AdminAIDrafts from '@/pages/AdminAIDrafts.vue'
import AIInsights from '@/pages/AIInsights.vue'
import PublicSite from '@/pages/PublicSite.vue'
import Promo from '@/pages/Promo.vue'
import Discussions from '@/pages/Discussions.vue'
import Risk from '@/pages/Risk.vue'
import ExamAnomalies from '@/pages/ExamAnomalies.vue'
import Settings from '@/pages/Settings.vue'
import Majors from '@/pages/Majors.vue'
import TeacherCourses from '@/pages/TeacherCourses.vue'
import TeacherGrade from '@/pages/TeacherGrade.vue'
import TeacherDiscussions from '@/pages/TeacherDiscussions.vue'
import Audit from '@/pages/Audit.vue'
import Placeholder from '@/pages/Placeholder.vue'
import { hasToken } from '@/api/client'

/// 路由表按 PRD v2 §8.1 的站点地图固化。
/// 权限由后端 RBAC + 资源归属校验把关（前端只做菜单可见性，不作为安全边界）。
const routes: RouteRecordRaw[] = [
  { path: '/admin/login', component: Login },
  { path: '/admin/change-password', component: ChangePassword },
  { path: '/', redirect: '/admin/courses' },
  { path: '/admin', redirect: '/admin/courses' },
  { path: '/admin/', redirect: '/admin/courses' },

  // —— 管理端（角色：管理员）——
  { path: '/admin/courses', component: Courses },
  { path: '/admin/content-requests', component: ContentRequests },
  { path: '/admin/operations', component: Operations },
  { path: '/admin/insights', component: Statistics },
  { path: '/admin/entitlements', component: Entitlements },
  { path: '/admin/appeals', component: Appeals },
  { path: '/admin/deletions', component: Deletions },
  { path: '/admin/reconcile', component: Reconcile },
  { path: '/admin/risk', component: Risk },
  { path: '/admin/exam-anomalies', component: ExamAnomalies },
  { path: '/admin/settings', component: Settings },
  { path: '/admin/majors', component: Majors },
  { path: '/admin/orders', component: Orders },
  { path: '/admin/refunds', component: Refunds },
  { path: '/admin/certs', component: Certs },
  { path: '/admin/comments', component: Comments },
  { path: '/admin/suggestions', component: Suggestions },
  { path: '/admin/kb', component: Knowledge },
  { path: '/admin/ai-drafts', component: AdminAIDrafts },
  { path: '/admin/ai-insights', component: AIInsights },
  { path: '/admin/public-site', component: PublicSite },
  { path: '/admin/promo', component: Promo },
  { path: '/admin/discussions', component: Discussions },
  { path: '/admin/broadcast', component: Broadcast },
  { path: '/admin/users', component: Users },
  { path: '/admin/staff', component: Staff },
  { path: '/admin/admins', component: Admins },
  { path: '/admin/audit', component: Audit },

  // —— 教师工作台（角色：教师）——
  { path: '/admin/teacher/courses', component: TeacherCourses },
  { path: '/admin/teacher/courses/new', component: Placeholder, props: { title: '新建课程', detail: '提交后需管理员审核上架' } },
  { path: '/admin/teacher/review', component: TeacherGrade },
  { path: '/admin/teacher/discussions', component: TeacherDiscussions },
  { path: '/admin/teacher/ai-drafts', component: TeacherAIDrafts },
  { path: '/admin/teacher/insights', component: TeacherInsights },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.path !== '/admin/login' && !hasToken()) return '/admin/login'
  if (to.path === '/admin/login' && hasToken()) return '/admin/courses'
  return true
})

