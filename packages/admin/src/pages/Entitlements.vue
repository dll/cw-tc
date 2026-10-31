<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listEntitlements, revokeEntitlement, type EntitlementItem } from '@/api/client'

const rows = ref<EntitlementItem[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 20
const filters = ref({ student_id: '', course_code: '', status: '', source: '' })

async function load() {
  loading.value = true
  try {
    const result = await listEntitlements({
      student_id: filters.value.student_id ? Number(filters.value.student_id) : undefined,
      course_code: filters.value.course_code || undefined,
      status: filters.value.status || undefined,
      source: filters.value.source || undefined,
      limit: pageSize,
      offset: (page.value - 1) * pageSize,
    })
    rows.value = result.items
    total.value = result.total
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '权益台账读取失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

const sourceLabel: Record<string, string> = { order: '订单购买', free: '免费课', grant: '管理员授权' }

async function revoke(row: EntitlementItem) {
  try {
    const { value } = await ElMessageBox.prompt('撤销授权只收权益，已发证书保留；离线下载同步失效。请填写事由（写审计）。', `撤销 ${row.student_no} · ${row.course_code}`, {
      confirmButtonText: '确认撤销',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    await ElMessageBox.confirm('确认执行撤销？该操作不可自动恢复。', '高危操作确认', { type: 'warning' })
    await revokeEntitlement(row.id, value.trim())
    ElMessage.success('权益已撤销并写审计')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>权益台账（学员标识脱敏 · 撤销须填事由并写审计）</template>
    <div style="display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 12px">
      <el-input v-model="filters.student_id" placeholder="学员 ID" clearable style="width: 130px" @keyup.enter="search" />
      <el-input v-model="filters.course_code" placeholder="课程码" clearable style="width: 180px" @keyup.enter="search" />
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 140px" @change="search">
        <el-option label="全部状态" value="" />
        <el-option label="有效" value="active" />
        <el-option label="已撤销" value="revoked" />
      </el-select>
      <el-select v-model="filters.source" placeholder="来源" clearable style="width: 150px" @change="search">
        <el-option label="全部来源" value="" />
        <el-option label="订单购买" value="order" />
        <el-option label="免费课" value="free" />
        <el-option label="管理员授权" value="grant" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="student_no" label="学号" width="130" />
      <el-table-column prop="student_name" label="姓名" width="110" />
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="course_name" label="课程" min-width="160" />
      <el-table-column label="来源" width="110">
        <template #default="{ row }">{{ sourceLabel[row.source] ?? row.source }}</template>
      </el-table-column>
      <el-table-column prop="order_no" label="订单号" min-width="150">
        <template #default="{ row }">{{ row.order_no ?? '-' }}</template>
      </el-table-column>
      <el-table-column prop="granted_at" label="开通时间" width="180" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">{{ row.status === 'active' ? '有效' : '已撤销' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="撤销" min-width="180">
        <template #default="{ row }">
          <span v-if="row.status !== 'active'">{{ row.revoke_reason ?? '-' }}</span>
          <el-button v-else size="small" type="danger" plain @click="revoke(row)">撤销权益</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 16px"
      @current-change="load"
    />
  </el-card>
</template>
