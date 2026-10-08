import axios, { type AxiosInstance } from 'axios'

/**
 * 管理端 API 客户端。
 *
 * 约定：
 *   - 后端统一信封 `{ code, data, msg }`，code !== 0 视为业务错误；
 *   - 会话 token 由 `/admin/login` 返回，存 localStorage，请求走 `Authorization: Bearer`；
 *   - 后台所有写操作后端都会记 `audit_log`，前端不得自行组装审计字段。
 */
export const http: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 20000,
  headers: { Accept: 'application/json' },
})

const TOKEN_KEY = 'cw_tc_admin_token'

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}
export function hasToken(): boolean {
  return !!localStorage.getItem(TOKEN_KEY)
}

http.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY)
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (res) => res,
  (err) => {
    const status = err?.response?.status
    if (status === 401) {
      clearToken()
      if (location.pathname !== '/login') location.href = '/login'
    }
    return Promise.reject(err)
  },
)

/** 统一取 data；code !== 0 抛业务错误。 */
async function unwrap<T>(p: Promise<{ data: { code: number; data?: T; msg?: string } }>): Promise<T> {
  const { data } = await p
  if (data.code !== 0) throw new Error(data.msg || `业务错误 ${data.code}`)
  return data.data as T
}

// —— 鉴权 ——
export interface AdminLoginResult {
  token: string
  account: string
  name: string
  role: string
}
export async function login(account: string, password: string): Promise<AdminLoginResult> {
  const res = await unwrap<AdminLoginResult>(http.post('/admin/login', { account, password }))
  saveToken(res.token)
  return res
}

// —— 台账（只读列表）——
export interface OrderItem {
  order_no: string
  phone: string
  course_name: string
  amount_cents: number
  status: string
  channel: string
  created_at: string
}
export interface RefundItem {
  refund_no: string
  order_no: string
  phone: string
  kind: string
  amount_cents: number
  reason: string
  status: string
  created_at: string
}
export interface CertItem {
  cert_no: string
  phone: string
  course: string
  level: string
  score: number
  band: string
  status: string
  issued_at: string
  expires_at: string
}
export async function listOrders(): Promise<OrderItem[]> {
  const r = await unwrap<{ items: OrderItem[] }>(http.get('/admin/orders'))
  return r.items
}
export async function listRefunds(): Promise<RefundItem[]> {
  const r = await unwrap<{ items: RefundItem[] }>(http.get('/admin/refunds'))
  return r.items
}
export async function listCerts(): Promise<CertItem[]> {
  const r = await unwrap<{ items: CertItem[] }>(http.get('/admin/certs'))
  return r.items
}

// —— 用户管理 ——
export interface UserItem {
  id: number
  student_no: string
  name: string
  phone: string
  status: string
  created_at: string
}
export async function listUsers(): Promise<UserItem[]> {
  const r = await unwrap<{ items: UserItem[] }>(http.get('/admin/users'))
  return r.items
}
export async function revealUser(id: number, reason: string): Promise<{ phone: string }> {
  return unwrap<{ phone: string }>(http.post(`/admin/users/${id}/reveal`, { reason }))
}
export async function grantEntitlement(input: {
  student_id: number
  course_code: string
  grant_reason: string
}): Promise<void> {
  await unwrap<unknown>(http.post('/admin/entitlements', input))
}
export async function revokeEntitlement(studentId: number, courseId: number): Promise<void> {
  await unwrap<unknown>(http.delete(`/admin/entitlements/${studentId}/${courseId}`))
}

// —— 广播 ——
export async function createBroadcast(input: {
  target: string
  target_value?: string
  title: string
  body: string
  link?: string
  idempotency_key: string
}): Promise<{ broadcast_id: number }> {
  return unwrap<{ broadcast_id: number }>(http.post('/admin/broadcasts', input))
}
export async function confirmBroadcast(id: number): Promise<{ recipients: number }> {
  return unwrap<{ recipients: number }>(http.post(`/admin/broadcasts/${id}/confirm`, {}))
}

// —— staff 管理 ——
export interface StaffItem {
  id: number
  account: string
  name: string
  role: string
}
export async function createStaff(input: {
  account: string
  name: string
  role: string
  password: string
}): Promise<{ id: number }> {
  return unwrap<{ id: number }>(http.post('/admin/staff', input))
}
export async function disableStaff(id: number): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/staff/${id}/disable`, {}))
}

// —— 课程管理 ——
export interface AdminCourse {
  course_code: string
  source: string
  name: string
  line: string
  status: string
  price_cents: number
  teacher_id: number | null
  created_at: string
}
export async function listAdminCourses(): Promise<AdminCourse[]> {
  const r = await unwrap<{ items: AdminCourse[] }>(http.get('/admin/courses'))
  return r.items
}
export async function reviewCourse(code: string, input: Record<string, unknown>): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/review`, input))
}
export async function publishCourse(code: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/publish`, {}))
}
export async function setPrice(code: string, priceCents: number, confirm: boolean): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/price`, { price_cents: priceCents, confirm }))
}

// —— 审计日志 ——
export interface AuditItem {
  id: number
  actor_type: string
  actor_id: number
  actor_role: string
  action: string
  target_type: string
  target_id: string
  created_at: string
}
export async function listAudit(action?: string): Promise<AuditItem[]> {
  const r = await unwrap<{ items: AuditItem[] }>(
    http.get('/admin/audit', { params: action ? { action } : undefined }),
  )
  return r.items
}

// —— 专业分类 ——
export interface MajorItem {
  id: number
  name: string
  sort: number
  status: string
}
export async function listMajors(): Promise<MajorItem[]> {
  const r = await unwrap<{ items: MajorItem[] }>(http.get('/admin/majors'))
  return r.items
}
export async function createMajor(name: string, sort: number): Promise<void> {
  await unwrap<unknown>(http.post('/admin/majors', { name, sort }))
}
export async function updateMajor(id: number, input: { name?: string; sort?: number; status?: string }): Promise<void> {
  await unwrap<unknown>(http.put(`/admin/majors/${id}`, input))
}

// —— 教师工作台 ——
export async function listTeacherCourses(): Promise<AdminCourse[]> {
  const r = await unwrap<{ items: AdminCourse[] }>(http.get('/teacher/courses'))
  return r.items
}
export async function createTeacherCourse(input: {
  name: string
  line?: string
  major_id?: number | null
  intro?: string
}): Promise<{ course_code: string }> {
  return unwrap<{ course_code: string }>(http.post('/teacher/courses', input))
}
export async function submitCourse(code: string): Promise<void> {
  await unwrap<unknown>(http.post(`/teacher/courses/${code}/submit`, {}))
}
