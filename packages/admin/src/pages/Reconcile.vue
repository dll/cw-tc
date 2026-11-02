<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  downloadReconcileDiff,
  listReconcile,
  paymentChannels,
  reconcileDay,
  type PaymentChannelStatus,
  type ReconcileItem,
} from '@/api/client'

const rows = ref<ReconcileItem[]>([])
const loading = ref(false)
const channel = ref('')
const day = ref('')
const dayRows = ref<ReconcileItem[]>([])
const dayBalanced = ref<boolean | null>(null)
const channels = ref<PaymentChannelStatus[]>([])

async function load() {
  loading.value = true
  try {
    const result = await listReconcile(channel.value)
    rows.value = result.items
    channels.value = await paymentChannels()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '对账结果读取失败')
  } finally {
    loading.value = false
  }
}

async function loadDay() {
  if (!day.value) return
  try {
    const result = await reconcileDay(day.value)
    dayRows.value = result.items
    dayBalanced.value = result.all_balanced
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '按日对账读取失败')
  }
}

async function downloadDiff() {
  if (!day.value) return
  try {
    const blob = await downloadReconcileDiff(day.value)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `reconcile-${day.value}-diff.csv`
    link.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '差异明细下载失败')
  }
}

function money(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>对账与支付渠道（差异必须为 0）</template>
    <el-descriptions :column="2" border size="small" style="margin-bottom: 12px">
      <el-descriptions-item v-for="item in channels" :key="item.channel" :label="item.channel === 'wechat' ? '微信支付' : '支付宝'">
        <el-tag :type="item.configured ? 'success' : 'info'" size="small">{{ item.configured ? '已配置' : '未配置' }}</el-tag>
        <span v-if="item.merchant_last4" style="margin-left: 8px">商户号后四位：{{ item.merchant_last4 }}</span>
      </el-descriptions-item>
    </el-descriptions>

    <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 12px; flex-wrap: wrap">
      <el-select v-model="channel" placeholder="渠道" clearable style="width: 150px" @change="load">
        <el-option label="全部渠道" value="" />
        <el-option label="微信支付" value="wechat" />
        <el-option label="支付宝" value="alipay" />
      </el-select>
      <el-date-picker v-model="day" type="date" value-format="YYYY-MM-DD" placeholder="选择对账日期" />
      <el-button type="primary" @click="loadDay">查看当日</el-button>
      <el-button @click="downloadDiff">下载差异 CSV</el-button>
    </div>

    <el-alert
      v-if="dayBalanced !== null"
      :type="dayBalanced ? 'success' : 'error'"
      :closable="false"
      :title="dayBalanced ? '当日对账差异为 0' : '当日存在差异，请立即核对（已触发告警口径）'"
      style="margin-bottom: 12px"
    />

    <el-table v-loading="loading" :data="dayRows.length ? dayRows : rows" stripe size="small">
      <el-table-column prop="biz_date" label="业务日期" width="120" />
      <el-table-column prop="channel" label="渠道" width="100" />
      <el-table-column label="渠道笔数 / 金额" min-width="180">
        <template #default="{ row }">{{ row.channel_count }} / {{ money(row.channel_amount_cents) }}</template>
      </el-table-column>
      <el-table-column label="本地笔数 / 金额" min-width="180">
        <template #default="{ row }">{{ row.local_count }} / {{ money(row.local_amount_cents) }}</template>
      </el-table-column>
      <el-table-column label="差异笔数 / 金额" min-width="180">
        <template #default="{ row }">
          <span :style="{ color: row.balanced ? 'inherit' : 'var(--el-color-danger)' }">
            {{ row.diff_count }} / {{ money(row.diff_amount_cents) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.balanced ? 'success' : 'danger'" size="small">{{ row.balanced ? '一致' : '有差异' }}</el-tag>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
