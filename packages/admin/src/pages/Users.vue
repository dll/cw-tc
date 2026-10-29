<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { listUsers, pendingRegistrationCount, revealUser, approveRegistration, grantEntitlement, batchDiscount, batchGrant, resetUserPassword, type UserItem, type UserType, type ResetPasswordResult } from '@/api/client'

const rows = ref<UserItem[]>([])
const loading = ref(false)
const userType = ref<UserType | ''>('')
const page = ref(1)
const total = ref(0)
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
    const result = await listUsers(userType.value, pageSize, (page.value - 1) * pageSize)
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
    <div style="margin-bottom: 16px">
      <el-select v-model="userType" placeholder="用户类型" style="width: 180px" @change="onTypeChange">
        <el-option label="全部类型" value="" />
        <el-option label="学生" value="student" />
        <el-option label="教师" value="teacher" />
        <el-option label="超级管理员" value="admin_super" />
        <el-option label="运营管理员" value="admin_ops" />
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
      <el-table-column label="操作" width="300">
        <template #default="{ row }">
          <template v-if="row.user_type === 'student'">
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
    <el-dialog v-model="resetVisible" title="重置密码（一次性重置码）" width="520px">
      <el-alert
        type="warning"
        :closable="false"
        title="请将重置码转交学员，由学员在「忘记密码」页面自行设置新密码；管理员不会获知新密码。"
        style="margin-bottom: 12px"
      />
      <el-descriptions v-if="resetResult" :column="1" border>
        <el-descriptions-item label="重置码">
          <span style="font-family: monospace; font-size: 20px; letter-spacing: 2px">{{ resetResult.reset_code }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="有效期至">{{ resetResult.expires_at }}</el-descriptions-item>
        <el-descriptions-item label="学员手机号后 4 位">{{ resetResult.phone_last4 }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="resetVisible = false">我已转交</el-button>
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
  </el-card>
</template>
