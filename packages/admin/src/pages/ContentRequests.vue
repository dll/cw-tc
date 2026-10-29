<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listContentRequests,
  approveContentRequest,
  rejectContentRequest,
  type ContentRequestItem,
} from '@/api/client'

const rows = ref<ContentRequestItem[]>([])
const loading = ref(false)
const status = ref('pending')

const kindLabel: Record<string, string> = {
  offline: '下架申请',
  urgent_fix: '紧急纠错',
}
const reasonLabel: Record<string, string> = {
  content_error: '内容错误',
  material_change: '教材变更',
  personal: '个人原因',
  other: '其他',
}
const statusLabel: Record<string, string> = {
  pending: '待处理',
  approved: '已批准',
  rejected: '已驳回',
}

async function load() {
  loading.value = true
  try {
    rows.value = await listContentRequests(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '申请列表加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function onApprove(row: ContentRequestItem) {
  try {
    await ElMessageBox.confirm(
      `批准后「${row.course_name}」将立即下架；已购学员仍可学习与测评。确认批准？`,
      '批准下架申请',
      { type: 'warning' },
    )
    await approveContentRequest(row.id)
    ElMessage.success('已批准并下架')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function onReject(row: ContentRequestItem) {
  try {
    const { value } = await ElMessageBox.prompt('驳回必须填写原因（会写审计）', '驳回申请', {
      confirmButtonText: '确认驳回',
      cancelButtonText: '取消',
      inputValidator: (v: string) => (v && v.trim() ? true : '原因必填'),
    })
    await rejectContentRequest(row.id, value.trim())
    ElMessage.success('已驳回')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <el-card>
    <template #header>教师下架 / 紧急纠错申请（批准后由管理员执行下架）</template>
    <el-form :inline="true" style="margin-bottom: 12px">
      <el-form-item label="状态">
        <el-select v-model="status" style="width: 140px" @change="load">
          <el-option label="待处理" value="pending" />
          <el-option label="已批准" value="approved" />
          <el-option label="已驳回" value="rejected" />
          <el-option label="全部" value="" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button @click="load">刷新</el-button>
      </el-form-item>
    </el-form>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="course_name" label="课程名称" min-width="180" />
      <el-table-column prop="teacher_name" label="教师" width="110" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }">{{ kindLabel[row.kind] ?? row.kind }}</template>
      </el-table-column>
      <el-table-column label="原因" width="130">
        <template #default="{ row }">{{ reasonLabel[row.reason_kind] ?? row.reason_kind }}</template>
      </el-table-column>
      <el-table-column prop="reason_text" label="说明" min-width="180" />
      <el-table-column prop="created_at" label="申请时间" width="170" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="操作" width="190">
        <template #default="{ row }">
          <template v-if="row.status === 'pending'">
            <el-button size="small" type="success" @click="onApprove(row)">批准并下架</el-button>
            <el-button size="small" type="danger" plain @click="onReject(row)">驳回</el-button>
          </template>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
