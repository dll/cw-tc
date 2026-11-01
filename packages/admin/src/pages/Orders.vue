<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { downloadOrdersCsv, listOrders, partialRefundOrder, type OrderItem } from '@/api/client'

const rows = ref<OrderItem[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 20
const filters = ref({ status: '', course_code: '', q: '', from: '', to: '' })
const refundVisible = ref(false)
const refundTarget = ref<OrderItem | null>(null)
const refundForm = ref({ amount_cents: 0, basis: 'campaign', basis_ref: '', reason: '' })

async function load() {
  loading.value = true
  try {
    const result = await listOrders({
      status: filters.value.status || undefined,
      course_code: filters.value.course_code || undefined,
      q: filters.value.q.trim() || undefined,
      from: filters.value.from || undefined,
      to: filters.value.to || undefined,
      limit: pageSize,
      offset: (page.value - 1) * pageSize,
    })
    rows.value = result.items
    total.value = result.total
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '订单加载失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

async function exportCsv() {
  try {
    const blob = await downloadOrdersCsv({
      status: filters.value.status || undefined,
      course_code: filters.value.course_code || undefined,
      q: filters.value.q.trim() || undefined,
      from: filters.value.from || undefined,
      to: filters.value.to || undefined,
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'orders.csv'
    link.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '导出失败')
  }
}

function openPartialRefund(row: OrderItem) {
  refundTarget.value = row
  refundForm.value = { amount_cents: row.amount_cents, basis: 'campaign', basis_ref: '', reason: '' }
  refundVisible.value = true
}

async function submitPartialRefund() {
  const target = refundTarget.value
  if (!target) return
  if (!refundForm.value.reason.trim() || !refundForm.value.basis_ref.trim() || refundForm.value.amount_cents <= 0) {
    ElMessage.warning('金额、依据、事由必填')
    return
  }
  try {
    await ElMessageBox.confirm('部分退款 = reward 打折：保留权益与证书、不计入退款率、写入审计。确认？', '高危操作确认', { type: 'warning' })
    await partialRefundOrder(target.order_no, {
      amount_cents: refundForm.value.amount_cents,
      basis: refundForm.value.basis,
      basis_ref: refundForm.value.basis_ref.trim(),
      reason: refundForm.value.reason.trim(),
    })
    refundVisible.value = false
    ElMessage.success('部分退款已完成')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>订单台账（手机号脱敏 · 支持筛选 / CSV / 部分退款）</template>
    <div style="display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 12px">
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 140px" @change="search">
        <el-option label="全部状态" value="" />
        <el-option label="待支付" value="pending" />
        <el-option label="已支付" value="paid" />
        <el-option label="已关闭" value="closed" />
        <el-option label="已退款" value="refunded" />
      </el-select>
      <el-input v-model="filters.course_code" placeholder="课程码" clearable style="width: 160px" @keyup.enter="search" />
      <el-input v-model="filters.q" placeholder="订单号 / 手机号 / 学号前缀" clearable style="width: 240px" @keyup.enter="search" />
      <el-date-picker v-model="filters.from" type="date" value-format="YYYY-MM-DD" placeholder="开始日期" />
      <el-date-picker v-model="filters.to" type="date" value-format="YYYY-MM-DD" placeholder="结束日期" />
      <el-button type="primary" @click="search">查询</el-button>
      <el-button @click="exportCsv">导出 CSV</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="order_no" label="订单号" width="170" />
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="student_no" label="学号" width="130" />
      <el-table-column prop="course_code" label="课程码" width="120" />
      <el-table-column prop="course_name" label="课程" min-width="160" />
      <el-table-column label="金额" width="100">
        <template #default="{ row }">¥{{ (row.amount_cents / 100).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="channel" label="渠道" width="90" />
      <el-table-column prop="paid_at" label="支付时间" width="180" />
      <el-table-column prop="created_at" label="下单时间" width="180" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button v-if="row.status === 'paid'" size="small" type="warning" plain @click="openPartialRefund(row)">部分退款</el-button>
          <span v-else>-</span>
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

    <el-dialog v-model="refundVisible" title="部分退款（reward 打折）" width="520px">
      <el-form label-position="top">
        <el-form-item label="订单">
          <el-input :model-value="refundTarget ? `${refundTarget.order_no} · ${refundTarget.course_name}` : ''" disabled />
        </el-form-item>
        <el-form-item label="退款金额（分）">
          <el-input-number v-model="refundForm.amount_cents" :min="1" :max="refundTarget?.amount_cents ?? 1" />
        </el-form-item>
        <el-form-item label="依据">
          <el-select v-model="refundForm.basis">
            <el-option label="成绩" value="score" />
            <el-option label="活动" value="campaign" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="依据引用（必填）"><el-input v-model="refundForm.basis_ref" /></el-form-item>
        <el-form-item label="事由（必填）"><el-input v-model="refundForm.reason" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="refundVisible = false">取消</el-button>
        <el-button type="danger" @click="submitPartialRefund">确认部分退款</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>
