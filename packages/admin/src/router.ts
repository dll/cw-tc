import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

import Placeholder from '@/pages/Placeholder.vue'

/// 路由表按 PRD v2 §8.1 的站点地图固化。
/// 管理端与教师工作台共用一个 SPA；权限由后端 RBAC + 资源归属校验把关
/// （前端只做菜单可见性，不作为安全边界）。
const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/admin/courses' },

  // —— 管理端（角色：管理员）——
  { path: '/admin/courses', component: Placeholder, props: { title: '课程管理', detail: '上架/下架 · 课程设置面板 · 审核教师提交' } },
  { path: '/admin/majors', component: Placeholder, props: { title: '专业分类', detail: '平台自有元数据，不来自上游' } },
  { path: '/admin/orders', component: Placeholder, props: { title: '订单与退款', detail: '退款须连带撤销证书，不可留中间态' } },
  { path: '/admin/certs', component: Placeholder, props: { title: '证书台账', detail: '签发 / 撤销 / 导出' } },
  { path: '/admin/kb', component: Placeholder, props: { title: '知识库重建', detail: '切片数与向量数须为真实统计' } },
  { path: '/admin/broadcast', component: Placeholder, props: { title: '广播通知', detail: '发送前预览触达人数' } },
  { path: '/admin/users', component: Placeholder, props: { title: '用户管理', detail: '学员 / 教师 / 管理员' } },
  { path: '/admin/audit', component: Placeholder, props: { title: '审计日志', detail: '资金与商业动作全部留痕' } },

  // —— 教师工作台（角色：教师）——
  { path: '/teacher/courses', component: Placeholder, props: { title: '我的课程', detail: '草稿 / 待审 / 已上架' } },
  { path: '/teacher/courses/new', component: Placeholder, props: { title: '新建课程', detail: '提交后需管理员审核上架' } },
  { path: '/teacher/review', component: Placeholder, props: { title: '阅卷台', detail: '主观题采点制' } },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})
