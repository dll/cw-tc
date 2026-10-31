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
export async function listUsers(
  userType: UserType | '' = '',
  limit = 20,
  offset = 0,
  q = '',
  status = '',
): Promise<{ items: UserItem[]; total: number }> {
  return unwrap<{ items: UserItem[]; total: number }>(http.get('/admin/users', {
    params: { type: userType, limit, offset, q, status },
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
export interface EntitlementItem {
  id: number
  student_no: string
  student_name: string
  course_code: string
  course_name: string
  source: string
  order_no: string | null
  grant_reason: string | null
  granted_at: string
  status: string
  revoked_at: string | null
  revoke_reason: string | null
}
export async function listEntitlements(params: {
  student_id?: number
  course_code?: string
  source?: string
  status?: string
  limit?: number
  offset?: number
} = {}): Promise<{ items: EntitlementItem[]; total: number }> {
  return unwrap<{ items: EntitlementItem[]; total: number }>(http.get('/admin/entitlements', { params }))
}
export async function revokeEntitlement(id: number, reason: string): Promise<void> {
  await unwrap<unknown>(http.delete(`/admin/entitlements/${id}`, { data: { reason, confirm: true } }))
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
  published_at: string
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
export interface OperationSlotItem {
  id: number
  position: string
  course_code: string
  title: string
  image_url: string
  link_url: string
  starts_at: string
  ends_at: string
  weight: number
  status: string
  created_by: number | null
}
export interface OperationSlotInput {
  position: string
  course_code: string
  title: string
  image_url: string
  link_url: string
  starts_at: string
  ends_at: string
  weight: number
  status: string
}
export async function listOperationSlots(): Promise<OperationSlotItem[]> {
  const r = await unwrap<{ items: OperationSlotItem[] }>(http.get('/admin/operations'))
  return r.items
}
export async function createOperationSlot(input: OperationSlotInput): Promise<{ id: number }> {
  return unwrap<{ id: number }>(http.post('/admin/operations', input))
}
export async function updateOperationSlot(id: number, input: OperationSlotInput): Promise<void> {
  await unwrap<unknown>(http.patch(`/admin/operations/${id}`, input))
}
export async function removeOperationSlot(id: number): Promise<void> {
  await unwrap<unknown>(http.delete(`/admin/operations/${id}`))
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
  content_updated_at: string
  published_at: string
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

export interface AdminComment {
  id: number
  course_code: string
  course_name: string
  nickname: string
  rating: number
  body: string
  status: string
  created_at: string
  pending_reports: number
}
export async function listAdminComments(status = 'visible'): Promise<AdminComment[]> {
  const data = await unwrap<{ items: AdminComment[] }>(http.get('/admin/comments', { params: { status } }))
  return data.items
}
export async function removeAdminComment(id: number, reasonKind: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/comments/${id}/remove`, { reason_kind: reasonKind, confirm: true }))
}

export interface AdminSuggestion {
  id: number
  student_id: number
  course_code: string | null
  body: string
  status: string
  reply: string | null
  created_at: string
}
export async function listAdminSuggestions(status = ''): Promise<AdminSuggestion[]> {
  const data = await unwrap<{ items: AdminSuggestion[] }>(http.get('/admin/suggestions', { params: { status } }))
  return data.items
}
export async function replyAdminSuggestion(id: number, status: string, reply: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/suggestions/${id}/reply`, { status, reply }))
}

// ── 系统配置（仅 super）与运营统计（模块 M） ──
export interface SystemConfigItem {
  key: string
  value: number | boolean | string | unknown
  value_type: string
  min_value: number | null
  max_value: number | null
  description: string
  updated_by: number | null
  updated_at: string
}
export async function listSettings(): Promise<SystemConfigItem[]> {
  const data = await unwrap<{ items: SystemConfigItem[] }>(http.get('/admin/settings'))
  return data.items
}
export async function updateSetting(key: string, value: number): Promise<SystemConfigItem> {
  return unwrap<SystemConfigItem>(http.patch('/admin/settings', { key, value }))
}

export interface CourseStats {
  course_code: string
  name?: string
  line?: string
  status?: string
  source?: string
  data_as_of: string | null
  views_uv: number
  favorites: number
  learning: number
  completed: number
  examined: number
  certified: number
  orders: number
  paid: number
  unlock: number
  avg_rating: number | null
  rating_count: number
}
export async function adminInsights(source = ''): Promise<CourseStats[]> {
  const data = await unwrap<{ items: CourseStats[] }>(http.get('/admin/insights', { params: { source } }))
  return data.items
}
export async function adminCourseStats(code: string): Promise<CourseStats & { funnel: { items: Array<{ key: string; label: string; value: number; rate_from_previous: number | null }> } }> {
  return unwrap(http.get(`/admin/stats/${code}`))
}
// ── 管理员：用户详情（默认脱敏；学习 / 测评 / 证书 / 订单 / AI 用量） ──
export interface AdminUserLearning {
  course_code: string
  name: string
  course_status: string
  lecture_count: number
  learned_lectures: number
  progress_percent: number
  last_lecture_no: number | null
  last_learned_at: string | null
  unlocked: boolean
}
export interface AdminUserExam {
  course_code: string
  course_name: string
  level: string
  total_score: number
  band: string | null
  is_pass: boolean
  exam_at: string
  cert_no: string | null
}
export interface AdminUserCertificate {
  cert_no: string
  course_code: string
  course_name: string
  level: string
  score: number
  band: string
  issued_at: string
  expires_at: string
  status: string
  revoked_reason: string | null
}
export interface AdminUserOrder {
  order_no: string
  course_name: string
  amount_cents: number
  status: string
  channel: string
  created_at: string
  paid_at: string | null
  refunded_at: string | null
}
export interface AdminUserDetail {
  user_type: string
  id: number
  name: string
  student_no?: string
  account?: string
  phone_masked?: string
  id_card_masked?: string
  status: string
  created_at: string
  registration_status?: string | null
  phone_verified_at?: string | null
  last_login_at?: string | null
  learning?: AdminUserLearning[]
  learning_summary?: { courses: number; completed_courses: number }
  exams?: AdminUserExam[]
  certificates?: AdminUserCertificate[]
  orders?: AdminUserOrder[]
  ai_usage?: { used_total: number; courses: number; last_date: string | null }
  courses?: Array<{ course_code: string; name: string; status: string }>
  note?: string
}
export async function getUserDetail(id: number, type: UserType): Promise<AdminUserDetail> {
  return unwrap<AdminUserDetail>(http.get(`/admin/users/${id}`, { params: { type } }))
}
export interface UserStatusInput {
  status: 'active' | 'suspended' | 'banned'
  reason_kind?: string
  note?: string
  until?: string
  confirm: true
}
export async function updateUserStatus(id: number, input: UserStatusInput): Promise<void> {
  await unwrap<unknown>(http.patch(`/admin/users/${id}/status`, input))
}
export async function revokeCertificate(certNo: string, reason: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/certs/${encodeURIComponent(certNo)}/revoke`, { reason, confirm: true }))
}
// ── 账号申诉 ──
export interface AppealItem {
  id: number
  phone_masked: string
  kind: string
  content_text: string
  status: string
  result_note: string | null
  student_no: string | null
  name: string | null
  created_at: string
  handled_at: string | null
}
export async function listAppeals(status = 'pending'): Promise<AppealItem[]> {
  const data = await unwrap<{ items: AppealItem[] }>(http.get('/admin/appeals', { params: { status } }))
  return data.items
}
export async function handleAppeal(id: number, status: 'accepted' | 'rejected', note: string): Promise<void> {
  await unwrap<unknown>(http.post(`/admin/appeals/${id}/handle`, { status, result_note: note, confirm: true }))
}