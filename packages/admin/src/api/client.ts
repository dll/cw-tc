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
const ROLE_KEY = 'cw_tc_admin_role'

export function saveToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}
export function saveRole(role: string) {
  localStorage.setItem(ROLE_KEY, role)
}
export function getRole(): string {
  return localStorage.getItem(ROLE_KEY) ?? ''
}
export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(ROLE_KEY)
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
    // 提取后端信封里的 msg，避免前端只显示 "Request failed with status code 401"
    const msg = err?.response?.data?.msg
    if (msg) return Promise.reject(new Error(msg))
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
  must_change_password: boolean
}
export async function login(account: string, password: string): Promise<AdminLoginResult> {
  const res = await unwrap<AdminLoginResult>(http.post('/admin/login', { account, password }))
  saveToken(res.token)
  saveRole(res.role)
  return res
}
export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await unwrap<unknown>(http.post('/admin/change-password', { old_password: oldPassword, new_password: newPassword }))
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
export type UserType = 'student' | 'teacher' | 'admin_super' | 'admin_ops'
export interface UserItem {
  id: number
  user_type: UserType
  student_no: string
  account: string
  name: string
  phone: string
  status: string
  created_at: string
  registration_status?: string
}
export async function approveRegistration(id: number, reason: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/users/${id}/approve-registration`, { reason, confirm: true }))
}
export async function pendingRegistrationCount(): Promise<number> {
  const result = await unwrap<{ count: number }>(http.get('/admin/registration-pending'))
  return result.count
}
export async function listUsers(userType: UserType | '' = '', limit = 20, offset = 0): Promise<{ items: UserItem[]; total: number }> {
  return unwrap<{ items: UserItem[]; total: number }>(http.get('/admin/users', {
    params: { type: userType, limit, offset },
  }))
}
export async function revealUser(id: number, reason: string): Promise<{ phone: string }> {
  return unwrap<{ phone: string }>(http.post(`/admin/users/${id}/reveal`, { reason }))
}
export interface ResetPasswordResult {
  reset_code: string
  expires_at: string
  phone_last4: string
  message: string
}
export async function resetUserPassword(id: number, reason: string): Promise<ResetPasswordResult> {
  return unwrap<ResetPasswordResult>(
    http.post(`/admin/users/${id}/reset-password`, { reason, confirm: true }),
  )
}
export async function grantEntitlement(input: {
  student_id: number
  course_code: string
  grant_reason: string
  confirm: boolean
}): Promise<void> {
  await unwrap<unknown>(http.post('/admin/entitlements', input))
}
export async function batchDiscount(input: {student_ids: number[]; course_code: string; discount_cents: number; reason: string; confirm: boolean}): Promise<{updated: number}> {
  return unwrap<{updated: number}>(http.post('/admin/student-discounts', input))
}
export async function batchGrant(input: {student_ids: number[]; course_code: string; reason: string; confirm: boolean}): Promise<{granted: number}> {
  return unwrap<{granted: number}>(http.post('/admin/student-grants', input))
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
  intro: string | null
  line: string
  lecture_count: number
  status: string
  price_cents: number
  teacher_id: number | null
  created_at: string
  updated_at: string
  major_id: number | null
  major_name: string | null
  free_preview_lectures: number
  sort_weight: number
  reviewed_at: string | null
  reject_reason: string | null
  ai_enabled: boolean
  kb_status: string
  content_version: number
  published_version: number | null
  has_unpublished_changes: boolean
}
export interface CourseRevision {
  revision_no: number
  content_version: number
  changed_digest: string
  published_at: string
}
export interface AdminCourseList {
  items: AdminCourse[]
  stats: Record<string, number>
  total: number
  total_lectures: number
}
export async function listAdminCourses(
  params: { status?: string; source?: string; q?: string } = {},
): Promise<AdminCourseList> {
  return unwrap<AdminCourseList>(http.get('/admin/courses', { params }))
}
export async function reviewCourse(code: string, input: Record<string, unknown>): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/review`, input))
}
export async function publishCourse(code: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/publish`, {}))
}
export async function unpublishCourse(code: string, reason: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/unpublish`, { reason, confirm: true }))
}
export interface BulkCourseResult {
  published?: number
  unpublished?: number
  failed: { course_code: string; reason: string }[]
}
export async function bulkPublishCourses(courseCodes: string[]): Promise<BulkCourseResult> {
  return unwrap<BulkCourseResult>(
    http.post('/admin/bulk-courses/publish', { course_codes: courseCodes, confirm: true }),
  )
}
export async function bulkUnpublishCourses(
  courseCodes: string[],
  reason: string,
): Promise<BulkCourseResult> {
  return unwrap<BulkCourseResult>(
    http.post('/admin/bulk-courses/unpublish', { course_codes: courseCodes, reason, confirm: true }),
  )
}
export interface ContentRequestItem {
  id: number
  course_code: string
  course_name: string
  teacher_id: number
  teacher_name: string
  kind: string
  reason_kind: string
  reason_text: string
  status: string
  handled_by: number | null
  handled_at: string | null
  created_at: string
}
export async function createContentRequest(
  code: string,
  input: { kind: string; reason_kind: string; reason_text: string },
): Promise<void> {
  await unwrap<unknown>(http.post(`/teacher/courses/${code}/requests`, input))
}
export async function listMyContentRequests(status = ''): Promise<ContentRequestItem[]> {
  const r = await unwrap<{ items: ContentRequestItem[] }>(
    http.get('/teacher/requests', { params: { status } }),
  )
  return r.items
}
export async function listContentRequests(status = ''): Promise<ContentRequestItem[]> {
  const r = await unwrap<{ items: ContentRequestItem[] }>(
    http.get('/admin/requests', { params: { status } }),
  )
  return r.items
}
export async function approveContentRequest(id: number): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/requests/${id}/approve`, { confirm: true }))
}
export async function rejectContentRequest(id: number, reason: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/requests/${id}/reject`, { reason, confirm: true }))
}
export async function listCourseRevisions(code: string): Promise<CourseRevision[]> {
  const r = await unwrap<{ items: CourseRevision[] }>(http.get(`/admin/courses/${code}/revisions`))
  return r.items
}
export interface CourseReviewDetail {
  round: number
  content_version: number
  reviewer_id: number
  reviewer_role: string
  reviewer_name: string | null
  score_material: number
  score_objective: number
  score_case: number
  score_lesson_plan: number
  score_slides: number
  score_lab: number
  total_score: number
  verdict: string
  issue_list: string | null
  reviewed_at: string
}
export interface CourseQuality {
  course_code: string
  status: string
  quality_score: number | null
  ai_level: string | null
  ai_enabled: boolean
  kb_status: string
  content_version: number
  published_version: number | null
  latest_review: CourseReviewDetail | null
}
export async function getCourseQuality(code: string): Promise<CourseQuality> {
  return unwrap<CourseQuality>(http.get(`/admin/courses/${code}/quality`))
}
export async function setCourseQuality(
  code: string,
  input: { quality_score?: number; ai_level?: string; reason: string; confirm: boolean },
): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/quality`, input))
}
export async function setPrice(code: string, priceCents: number, confirm: boolean): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/price`, { price_cents: priceCents, confirm }))
}
export async function setCourseMajor(code: string, majorId: number): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/major`, { major_id: majorId }))
}
export async function setCourseFreePreview(code: string, count: number): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/courses/${code}/free-preview`, { count }))
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
export async function listPublicMajors(): Promise<MajorItem[]> {
  const r = await unwrap<{ items: MajorItem[] }>(http.get('/majors'))
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
  course_code: string
  name: string
  line?: string
  major_id?: number | null
  intro?: string
}): Promise<{ course_code: string }> {
  return unwrap<{ course_code: string }>(http.post('/teacher/courses', input))
}
export async function updateTeacherCourse(
  code: string,
  input: { name: string; line?: string; major_id?: number | null; intro?: string },
): Promise<void> {
  await unwrap<unknown>(http.put(`/teacher/courses/${code}`, input))
}
export async function deleteTeacherCourse(code: string): Promise<void> {
  await unwrap<unknown>(http.delete(`/teacher/courses/${code}`))
}
export async function submitCourse(code: string): Promise<void> {
  await unwrap<unknown>(http.post(`/teacher/courses/${code}/submit`, {}))
}

// —— 阅卷台 ——
export interface GradeListItem {
  ResultID: number
  ExamSessionID: number
  StudentID: number
  CourseID: number
  CourseName: string
  Level: string
  AutoScore: number
  ManualScore: number
  TotalScore: number
}
export interface GradeQuestion {
  qno: string
  question_id: number
  stem: string
  score: number
  gradability: string
  answer: string
}
export async function listGradeReviews(): Promise<GradeListItem[]> {
  const r = await unwrap<{ items: GradeListItem[] }>(http.get('/teacher/grade'))
  return r.items
}
export async function gradeDetail(resultId: number): Promise<{ result_id: number; items: GradeQuestion[] }> {
  return unwrap<{ result_id: number; items: GradeQuestion[] }>(http.get(`/teacher/grade/${resultId}`))
}
export async function submitGrade(resultId: number, questionId: number, input: { score: number; comment?: string; points?: string }): Promise<void> {
  await unwrap<unknown>(http.post(`/teacher/grade/${resultId}/questions/${questionId}`, input))
}

