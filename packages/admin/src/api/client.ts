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
