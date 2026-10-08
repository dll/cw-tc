<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listCerts, type CertItem } from '@/api/client'

const rows = ref<CertItem[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    rows.value = await listCerts()
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <el-card>
    <template #header>证书台账（手机号脱敏）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="cert_no" label="证书编号" width="180" />
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="course" label="课程" width="120" />
      <el-table-column prop="level" label="等级" width="80" />
      <el-table-column prop="score" label="分数" width="80" />
      <el-table-column prop="band" label="评定" width="90" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column prop="issued_at" label="签发时间" width="180" />
      <el-table-column prop="expires_at" label="有效期至" width="180" />
    </el-table>
  </el-card>
</template>
