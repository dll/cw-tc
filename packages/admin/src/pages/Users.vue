<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { listUsers, revealUser, grantEntitlement, type UserItem, type UserType } from '@/api/client'

const rows = ref<UserItem[]>([])
const loading = ref(false)
const userType = ref<UserType | ''>('')
const page = ref(1)
const total = ref(0)
const pageSize = 20

async function loadUsers() {
  loading.value = true
  try {
    const result = await listUsers(userType.value, pageSize, (page.value - 1) * pageSize)
    rows.value = result.items
    total.value = result.total
  } finally {
    loading.value = false
  }
}

function onTypeChange() {
  page.value = 1
  void loadUsers()
}

onMounted(loadUsers)

const userTypeLabels: Record<UserType, string> = {
  student: '学生',
  teacher: '教师',
  admin_super: '超级管理员',
  admin_ops: '运营管理员',
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
    <div style="margin-bottom: 16px">
      <el-select v-model="userType" placeholder="用户类型" style="width: 180px" @change="onTypeChange">
        <el-option label="全部类型" value="" />
        <el-option label="学生" value="student" />
        <el-option label="教师" value="teacher" />
        <el-option label="超级管理员" value="admin_super" />
        <el-option label="运营管理员" value="admin_ops" />
      </el-select>
    </div>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="用户类型" width="130">
        <template #default="{ row }">{{ userTypeLabels[(row as UserItem).user_type] }}</template>
      </el-table-column>
      <el-table-column prop="student_no" label="学号" width="120" />
      <el-table-column prop="account" label="账号" width="130" />
      <el-table-column prop="name" label="姓名" width="120" />
      <el-table-column prop="phone" label="手机号" width="140" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="created_at" label="注册时间" width="180" />
      <el-table-column label="操作" width="200">
        <template #default="{ row }">
          <template v-if="row.user_type === 'student'">
            <el-button size="small" @click="onReveal(row)">看全号</el-button>
            <el-button size="small" type="warning" @click="openGrant(row)">授权学习</el-button>
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
  </el-card>
</template>
