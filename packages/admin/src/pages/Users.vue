<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { listUsers, pendingRegistrationCount, revealUser, approveRegistration, grantEntitlement, batchDiscount, batchGrant, resetUserPassword, getUserDetail, updateUserStatus, adminExportStudent, type UserItem, type UserType, type ResetPasswordResult, type AdminUserDetail } from '@/api/client'

const rows = ref<UserItem[]>([])
const loading = ref(false)
const userType = ref<UserType | ''>('')
const page = ref(1)
const total = ref(0)
const q = ref('')
const statusFilter = ref('')
const pendingCount = ref(0)
const loadError = ref('')
const selected = ref<UserItem[]>([])
const batchVisible = ref(false)
const batchForm = ref({ course_code: '', reason: '', discount_cents: 0, mode: 'discount' })
const pageSize = 20

async function loadUsers() {
  loading.value = true
  loadError.value = ''
  try {
    const result = await listUsers(userType.value, pageSize, (page.value - 1) * pageSize, q.value.trim(), statusFilter.value)
    rows.value = result.items
    total.value = result.total
    pendingCount.value = await pendingRegistrationCount()
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '用户列表加载失败'
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onTypeChange() {
  page.value = 1
  void loadUsers()
}

function onFilterChange() {
  page.value = 1
  void loadUsers()
}

onMounted(loadUsers)

async function submitBatch() {
  const ids = selected.value.filter((row) => row.user_type === 'student').map((row) => row.id)
  if (!ids.length || !batchForm.value.course_code || !batchForm.value.reason) return
  try {
    await ElMessageBox.confirm(`确认审批 ${ids.length} 名学员？此操作将写入审计记录。`, '二次确认', { type: 'warning' })
    const input = {student_ids: ids, course_code: batchForm.value.course_code, reason: batchForm.value.reason, confirm: true}
    const result = batchForm.value.mode === 'free'
      ? await batchGrant(input)
      : await batchDiscount({...input, discount_cents: batchForm.value.discount_cents})
    batchVisible.value = false
    ElMessageBox.alert(`已处理 ${'granted' in result ? result.granted : result.updated} 名学员`, '审批结果')
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessageBox.alert(e.message, '审批失败')
  }
}

const userTypeLabels: Record<UserType, string> = {
  student: '学生',
  teacher: '教师',
  admin_super: '超级管理员',
  admin_ops: '运营管理员',
}

// 账号统一展示：学员用手机号（登录账号），内部人员用账号；不回退学号。
function accountLabel(row: UserItem): string {
  return row.phone || row.account || '—'
}

async function onReveal(row: UserItem) {
  try {
    const { value } = await ElMessageBox.prompt('看全号必须填写事由（每次都会写审计）', `查看 ${row.name} 手机号`, {
      confirmButtonText: '确认查看',
      cancelButtonText: '取消',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    const { phone } = await revealUser(row.id, value.trim())
    ElMessageBox.alert(phone, '手机号全号', { confirmButtonText: '关闭' })
  } catch {
    /* 用户取消 */
  }
}

async function onApproveRegistration(row: UserItem) {
  try {
    const { value } = await ElMessageBox.prompt('审核手机号必须填写事由', '审核注册', {
      confirmButtonText: '通过审核', cancelButtonText: '取消',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    await approveRegistration(row.id, value.trim())
    await loadUsers()
    ElMessageBox.alert('审核通过，学员现在可以登录', '审核完成')
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessageBox.alert(e.message, '审核失败')
  }
}

const resetVisible = ref(false)
const resetResult = ref<ResetPasswordResult | null>(null)

const statusVisible = ref(false)
const statusTarget = ref<UserItem | null>(null)
const statusForm = ref({ status: 'suspended' as 'active' | 'suspended' | 'banned', reason_kind: 'payment_abuse', note: '', until: '' })

function openStatus(row: UserItem) {
  statusTarget.value = row
  const active = row.status === 'active'
  statusForm.value = {
    status: active ? 'suspended' : 'active',
    reason_kind: 'payment_abuse',
    note: '',
    until: active ? new Date(Date.now() + 72 * 3600 * 1000).toISOString().slice(0, 16) : '',
  }
  statusVisible.value = true
}

async function submitStatus() {
  const target = statusTarget.value
  if (!target) return
  const form = statusForm.value
  try {
    if (form.status !== 'active' && !form.note.trim()) return
    const until = form.status === 'active' || !form.until ? undefined : new Date(form.until).toISOString()
    await ElMessageBox.confirm(
      form.status === 'active' ? `确认解封 ${target.name}？已撤销证书与已退款订单不会恢复。` : `确认将 ${target.name} 设为${form.status === 'banned' ? '封禁' : '停用'}？将立即踢出全部登录设备并写审计。`,
      '高危操作确认',
      { type: 'warning' },
    )
    await updateUserStatus(target.id, {
      status: form.status,
      reason_kind: form.status === 'active' ? undefined : form.reason_kind,
      note: form.note.trim() || undefined,
      until,
      confirm: true,
    })
    statusVisible.value = false
    await loadUsers()
    ElMessageBox.alert('状态已更新并写入审计', '已生效')
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessageBox.alert(e.message, '操作失败')
  }
}
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<AdminUserDetail | null>(null)


async function exportStudent() {
  const target = detail.value
  if (!target) return
  try {
    const { value } = await ElMessageBox.prompt('代学员导出仅 super 可执行；须填写事由（写审计与导出留痕），内容自动脱敏。', '代学员导出数据', {
      confirmButtonText: '生成导出',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    const result = await adminExportStudent(target.id, 'all', value.trim())
    const blob = new Blob([JSON.stringify(result.data, null, 2)], { type: 'application/json;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `student-${target.student_no ?? target.id}-export.json`
    link.click()
    URL.revokeObjectURL(url)
    ElMessageBox.alert(`导出已生成并留痕（导出编号 ${result.export_id}）。`, '导出完成')
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessageBox.alert(e.message, '导出失败')
  }
}
async function openDetail(row: UserItem) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await getUserDetail(row.id, row.user_type)
  } catch (e) {
    detailVisible.value = false
    ElMessageBox.alert(e instanceof Error ? e.message : '用户详情读取失败', '读取失败')
  } finally {
    detailLoading.value = false
  }
}

async function onResetPassword(row: UserItem) {
  try {
    const { value } = await ElMessageBox.prompt('重置密码必须填写事由（会写审计）', `重置 ${row.name} 的密码`, {
      confirmButtonText: '生成重置码',
      cancelButtonText: '取消',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    await ElMessageBox.confirm('重置码有效期 30 分钟，学员使用后旧密码立即失效。确认生成？', '二次确认', { type: 'warning' })
    resetResult.value = await resetUserPassword(row.id, value.trim())
    resetVisible.value = true
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessageBox.alert(e.message, '重置失败')
  }
}

const grantVisible = ref(false)
const grantTarget = ref<UserItem | null>(null)
const grantForm = ref({ course_code: '', grant_reason: '' })

function openGrant(row: UserItem) {
  grantTarget.value = row
  grantForm.value = { course_code: '', grant_reason: '' }
  grantVisible.value = true
}

async function submitGrant() {
  const target = grantTarget.value
  if (!target) return
  if (!grantForm.value.course_code || !grantForm.value.grant_reason) {
    return
  }
  try {
    await grantEntitlement({
      student_id: target.id,
      course_code: grantForm.value.course_code,
      grant_reason: grantForm.value.grant_reason,
      confirm: true,
    })
    grantVisible.value = false
    ElMessageBox.alert('授权成功（先买后退，refund.kind=reward，不计退款率）', '授权免费学习', { confirmButtonText: '关闭' })
  } catch (e) {
    ElMessageBox.alert(e instanceof Error ? e.message : '授权失败', '错误', { confirmButtonText: '关闭' })
  }
}
</script>

<template>
  <el-card>
    <template #header>用户管理（默认脱敏，看全号需事由并写审计）</template>
    <el-alert v-if="loadError" :title="loadError" type="error" show-icon style="margin-bottom: 12px" />
    <el-alert v-if="pendingCount > 0" :title="`有 ${pendingCount} 名新学员等待注册审核`" type="warning" show-icon style="margin-bottom: 12px" />
    <div style="margin-bottom: 16px; display: flex; gap: 12px; flex-wrap: wrap">
      <el-input
        v-model="q"
        placeholder="手机号 / 姓名 / 学号 / 账号（前缀）"
        clearable
        style="width: 280px"
        @keyup.enter="onFilterChange"
        @clear="onFilterChange"
      />
      <el-button @click="onFilterChange">搜索</el-button>
      <el-select v-model="userType" placeholder="用户类型" style="width: 180px" @change="onTypeChange">
        <el-option label="全部类型" value="" />
        <el-option label="学生" value="student" />
        <el-option label="教师" value="teacher" />
        <el-option label="超级管理员" value="admin_super" />
        <el-option label="运营管理员" value="admin_ops" />
      </el-select>
      <el-select v-model="statusFilter" placeholder="状态" clearable style="width: 150px" @change="onFilterChange">
        <el-option label="全部状态" value="" />
        <el-option label="正常" value="active" />
        <el-option label="临时限制" value="limited" />
        <el-option label="停用" value="suspended" />
        <el-option label="永久封禁" value="banned" />
        <el-option label="已停用（内部）" value="disabled" />
      </el-select>
    </div>
    <el-button type="primary" :disabled="!selected.length" @click="batchVisible = true">批量折扣 / 免费学习</el-button>
    <el-table v-loading="loading" :data="rows" stripe @selection-change="(items: UserItem[]) => selected = items">
      <el-table-column type="selection" width="50" />
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="用户类型" width="130">
        <template #default="{ row }">{{ userTypeLabels[(row as UserItem).user_type] }}</template>
      </el-table-column>
      <el-table-column label="账号" min-width="160">
        <template #default="{ row }">{{ accountLabel(row as UserItem) }}</template>
      </el-table-column>
      <el-table-column prop="name" label="姓名" width="120" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="registration_status" label="注册审核" width="110" />
      <el-table-column prop="created_at" label="注册时间" width="180" />
      <el-table-column label="操作" width="470">
        <template #default="{ row }">
          <el-button size="small" type="primary" plain @click="openDetail(row)">详情</el-button>
          <template v-if="row.user_type === 'student'">
            <el-button size="small" type="warning" plain @click="openStatus(row)">{{ row.status === 'active' ? '停用/封禁' : '解封' }}</el-button>
            <el-button size="small" @click="onReveal(row)">看全号</el-button>
            <el-button v-if="row.registration_status === 'pending'" size="small" type="success" @click="onApproveRegistration(row)">审核通过</el-button>
            <el-button size="small" type="warning" @click="openGrant(row)">授权学习</el-button>
            <el-button size="small" type="danger" plain @click="onResetPassword(row)">重置密码</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 16px"
      @current-change="loadUsers"
    />

    <el-dialog v-model="grantVisible" title="授权免费学习（先买后退）" width="480px">
      <el-form label-position="top">
        <el-form-item label="课程码">
          <el-input v-model="grantForm.course_code" placeholder="如 upstream-ai-001" />
        </el-form-item>
        <el-form-item label="事由（必填，写审计）">
          <el-input v-model="grantForm.grant_reason" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="grantVisible = false">取消</el-button>
        <el-button type="primary" @click="submitGrant">确认授权</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="resetVisible" title="密码重置已授权" width="540px">
      <el-alert
        type="success"
        :closable="false"
        title="重置码已通过系统站内消息发送给学员（登录后可在「我的 → 消息」查看）；学员也可在登录页「忘记密码」中用身份证验证直接重置。请勿通过微信/电话等站外渠道转交。"
        style="margin-bottom: 12px"
      />
      <el-descriptions v-if="resetResult" :column="1" border>
        <el-descriptions-item label="有效期至">{{ resetResult.expires_at }}</el-descriptions-item>
        <el-descriptions-item label="学员手机号后 4 位">{{ resetResult.phone_last4 }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="resetVisible = false">关闭</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="statusVisible" title="账号状态（高危操作，后台二次确认 + 审计）" width="520px">
      <el-form label-position="top">
        <el-form-item label="目标用户">
          <el-input :model-value="statusTarget ? `${statusTarget.name}（${statusTarget.student_no || statusTarget.id}）` : ''" disabled />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="statusForm.status">
            <el-option label="停用（可解封）" value="suspended" />
            <el-option label="封禁（不填期限 = 永久，仅 super）" value="banned" />
            <el-option label="解封 / 恢复正常" value="active" />
          </el-select>
        </el-form-item>
        <template v-if="statusForm.status !== 'active'">
          <el-form-item label="原因类别">
            <el-select v-model="statusForm.reason_kind">
              <el-option label="短信接口滥用" value="sms_abuse" />
              <el-option label="支付异常 / 恶意退款" value="payment_abuse" />
              <el-option label="实名不符" value="identity_mismatch" />
              <el-option label="测评异常" value="exam_fraud" />
              <el-option label="严重违规" value="serious_violation" />
              <el-option label="登录失败过多" value="login_fail" />
            </el-select>
          </el-form-item>
          <el-form-item label="说明（必填，≤500 字）">
            <el-input v-model="statusForm.note" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item label="期限（停用必填；封禁留空 = 永久）">
            <el-date-picker v-model="statusForm.until" type="datetime" value-format="YYYY-MM-DDTHH:mm" placeholder="选择期限" />
          </el-form-item>
        </template>
        <el-form-item v-else label="解封说明（选填）">
          <el-input v-model="statusForm.note" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="statusVisible = false">取消</el-button>
        <el-button type="danger" @click="submitStatus">确认变更</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="batchVisible" title="批量审批学习优惠" width="480px">
      <el-form label-position="top">
        <el-form-item label="方式"><el-select v-model="batchForm.mode"><el-option label="折扣" value="discount" /><el-option label="免费学习" value="free" /></el-select></el-form-item>
        <el-form-item label="课程码"><el-input v-model="batchForm.course_code" /></el-form-item>
        <el-form-item v-if="batchForm.mode === 'discount'" label="减免金额（分）"><el-input-number v-model="batchForm.discount_cents" :min="0" /></el-form-item>
        <el-form-item label="审批事由"><el-input v-model="batchForm.reason" type="textarea" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="batchVisible = false">取消</el-button><el-button type="primary" @click="submitBatch">提交审批</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="用户详情（默认脱敏）" size="70%">
      <div v-loading="detailLoading">
        <template v-if="detail">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="用户类型">{{ userTypeLabels[detail.user_type as UserType] ?? detail.user_type }}</el-descriptions-item>
            <el-descriptions-item label="ID">{{ detail.id }}</el-descriptions-item>
            <el-descriptions-item label="姓名">{{ detail.name }}</el-descriptions-item>
            <el-descriptions-item label="学号 / 账号">{{ detail.student_no || detail.account || '-' }}</el-descriptions-item>
            <el-descriptions-item label="手机号">{{ detail.phone_masked || '-' }}</el-descriptions-item>
            <el-descriptions-item label="身份证号">{{ detail.id_card_masked || '-' }}</el-descriptions-item>
            <el-descriptions-item label="状态">{{ detail.status }}</el-descriptions-item>
            <el-descriptions-item label="注册时间">{{ detail.created_at }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.registration_status" label="注册审核">{{ detail.registration_status }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.last_login_at" label="最近登录">{{ detail.last_login_at }}</el-descriptions-item>
          </el-descriptions>

          <div style="margin-top: 12px">
            <el-button v-if="detail.user_type === 'student'" type="warning" plain @click="exportStudent">代学员导出数据（仅 super）</el-button>
          </div>

          <el-alert
            v-if="detail.note"
            type="info"
            :closable="false"
            :title="detail.note"
            style="margin: 12px 0"
          />

          <template v-if="detail.user_type === 'student'">
            <h3 style="margin: 16px 0 8px">学习进度</h3>
            <el-alert
              type="success"
              :closable="false"
              :title="`共 ${detail.learning_summary?.courses ?? 0} 门课，已学完 ${detail.learning_summary?.completed_courses ?? 0} 门`"
              style="margin-bottom: 8px"
            />
            <el-table :data="detail.learning ?? []" size="small">
              <el-table-column prop="course_code" label="课程码" width="120" />
              <el-table-column prop="name" label="课程" min-width="160" />
              <el-table-column label="进度" min-width="180">
                <template #default="{ row }">
                  <el-progress :percentage="row.progress_percent" :status="row.progress_percent === 100 ? 'success' : undefined" />
                  <span style="font-size: 12px; color: var(--el-text-color-secondary)">
                    {{ row.learned_lectures }} / {{ row.lecture_count }} 讲
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="last_lecture_no" label="最近讲次" width="100">
                <template #default="{ row }">{{ row.last_lecture_no ?? '-' }}</template>
              </el-table-column>
              <el-table-column prop="last_learned_at" label="最近学习" width="180">
                <template #default="{ row }">{{ row.last_learned_at ?? '-' }}</template>
              </el-table-column>
              <el-table-column label="权益" width="90">
                <template #default="{ row }">
                  <el-tag :type="row.unlocked ? 'success' : 'info'" size="small">{{ row.unlocked ? '有效' : '无' }}</el-tag>
                </template>
              </el-table-column>
            </el-table>

            <h3 style="margin: 16px 0 8px">测评记录</h3>
            <el-table :data="detail.exams ?? []" size="small">
              <el-table-column prop="course_name" label="课程" min-width="140" />
              <el-table-column prop="level" label="等级" width="70" />
              <el-table-column prop="total_score" label="总分" width="80" />
              <el-table-column prop="band" label="档位" width="90" />
              <el-table-column label="通过" width="70">
                <template #default="{ row }">{{ row.is_pass ? '是' : '否' }}</template>
              </el-table-column>
              <el-table-column prop="exam_at" label="测评时间" width="180" />
              <el-table-column prop="cert_no" label="证书编号" min-width="160">
                <template #default="{ row }">{{ row.cert_no ?? '-' }}</template>
              </el-table-column>
            </el-table>

            <h3 style="margin: 16px 0 8px">证书</h3>
            <el-table :data="detail.certificates ?? []" size="small">
              <el-table-column prop="cert_no" label="证书编号" min-width="170" />
              <el-table-column prop="course_name" label="课程" min-width="140" />
              <el-table-column prop="level" label="等级" width="70" />
              <el-table-column prop="score" label="成绩" width="80" />
              <el-table-column prop="status" label="状态" width="90" />
              <el-table-column prop="issued_at" label="签发时间" width="180" />
              <el-table-column prop="revoked_reason" label="撤销原因" width="130">
                <template #default="{ row }">{{ row.revoked_reason ?? '-' }}</template>
              </el-table-column>
            </el-table>

            <h3 style="margin: 16px 0 8px">订单与退款</h3>
            <el-table :data="detail.orders ?? []" size="small">
              <el-table-column prop="order_no" label="订单号" min-width="170" />
              <el-table-column prop="course_name" label="课程" min-width="140" />
              <el-table-column label="金额" width="110">
                <template #default="{ row }">¥{{ (row.amount_cents / 100).toFixed(2) }}</template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="100" />
              <el-table-column prop="channel" label="渠道" width="90" />
              <el-table-column prop="created_at" label="下单时间" width="180" />
              <el-table-column prop="refunded_at" label="退款时间" width="180">
                <template #default="{ row }">{{ row.refunded_at ?? '-' }}</template>
              </el-table-column>
            </el-table>

            <h3 style="margin: 16px 0 8px">AI 用量</h3>
            <el-descriptions :column="3" border size="small">
              <el-descriptions-item label="累计提问">{{ detail.ai_usage?.used_total ?? 0 }} 次</el-descriptions-item>
              <el-descriptions-item label="涉及课程">{{ detail.ai_usage?.courses ?? 0 }} 门</el-descriptions-item>
              <el-descriptions-item label="最近使用">{{ detail.ai_usage?.last_date ?? '-' }}</el-descriptions-item>
            </el-descriptions>
          </template>

          <template v-else>
            <h3 style="margin: 16px 0 8px">账号信息</h3>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="登录账号">{{ detail.account ?? '-' }}</el-descriptions-item>
              <el-descriptions-item label="状态">{{ detail.status }}</el-descriptions-item>
            </el-descriptions>
            <template v-if="detail.courses?.length">
              <h3 style="margin: 16px 0 8px">负责课程</h3>
              <el-table :data="detail.courses" size="small">
                <el-table-column prop="course_code" label="课程码" width="140" />
                <el-table-column prop="name" label="课程" min-width="180" />
                <el-table-column prop="status" label="状态" width="120" />
              </el-table>
            </template>
          </template>
        </template>
      </div>
    </el-drawer>
  </el-card>
</template>
