<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listOrders, type OrderItem } from '@/api/client'

const rows = ref<OrderItem[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    rows.value = await listOrders()
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <el-card>
    <template #header>订单台账（手机号脱敏）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="order_no" label="订单号" width="180" />
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="course_name" label="课程" />
      <el-table-column label="金额" width="100">
        <template #default="{ row }">¥{{ (row.amount_cents / 100).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="channel" label="渠道" width="90" />
      <el-table-column prop="created_at" label="创建时间" width="180" />
    </el-table>
  </el-card>
</template>
