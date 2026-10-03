import { describe, expect, it } from 'vitest'

import { router } from './router'

/**
 * 路由表 = PRD v2 §8.1 站点地图的代码化。
 * 这条测试的作用是：站点地图改了而路由没跟上（或反之）时立刻暴露。
 */
const EXPECTED_PATHS = [
  // 管理端（PRD v2 §7.10）
  '/admin/courses',
  '/admin/majors',
  '/admin/orders',
  '/admin/certs',
  '/admin/kb',
  '/admin/broadcast',
  '/admin/users',
  '/admin/audit',
  // 教师工作台（PRD v2 §7.9）
  '/teacher/courses',
  '/teacher/courses/new',
  '/teacher/review',
]

describe('路由表与 PRD v2 §8.1 站点地图一致', () => {
  it('关键路径均已注册', () => {
    const registered = router.getRoutes().map((r) => r.path)
    const missing = EXPECTED_PATHS.filter((p) => !registered.includes(p))
    expect(missing).toEqual([])
  })

  it('根路径重定向到课程管理', () => {
    const root = router.getRoutes().find((r) => r.path === '/')
    expect(root?.redirect).toBe('/admin/courses')
  })
})
