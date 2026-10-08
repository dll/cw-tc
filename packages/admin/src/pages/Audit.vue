<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listAudit, type AuditItem } from '@/api/client'

const rows = ref<AuditItem[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    rows.value = await listAudit()
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <el-card>
    <template #header>审计日志（只增不改不删）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="action" label="动作" width="200" />
      <el-table-column prop="actor_role" label="角色" width="110" />
      <el-table-column prop="actor_id" label="操作人" width="90" />
      <el-table-column prop="target_type" label="对象类型" width="110" />
      <el-table-column prop="target_id" label="对象" width="140" />
      <el-table-column prop="created_at" label="时间" width="180" />
    </el-table>
  </el-card>
</template>
