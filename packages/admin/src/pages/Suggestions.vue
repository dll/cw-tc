<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listAdminSuggestions, replyAdminSuggestion, type AdminSuggestion } from '@/api/client'

const rows = ref<AdminSuggestion[]>([])
const loading = ref(false)
const status = ref('pending')

async function load() {
  loading.value = true
  try { rows.value = await listAdminSuggestions(status.value) } catch (e) { ElMessage.error(e instanceof Error ? e.message : '加载失败') } finally { loading.value = false }
}

async function reply(row: AdminSuggestion) {
  try {
    const { value } = await ElMessageBox.prompt('回复学员（采纳需注明版本）', '处理建议', { inputType: 'textarea', inputPattern: /\S+/ })
    const result = await ElMessageBox.confirm('选择处理结果', '处理建议', { distinguishCancelAndClose: true, confirmButtonText: '采纳', cancelButtonText: '不采纳' }).then(() => 'accepted').catch(() => 'rejected')
    await replyAdminSuggestion(row.id, result, value)
    ElMessage.success('已回复')
    await load()
  } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error(e instanceof Error ? e.message : '处理失败') }
}

onMounted(load)
</script>

<template>
  <div>
    <el-radio-group v-model="status" @change="load" style="margin-bottom: 12px">
      <el-radio-button label="pending">待处理</el-radio-button>
      <el-radio-button label="">全部</el-radio-button>
    </el-radio-group>
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="body" label="建议" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="reply" label="回复" />
      <el-table-column label="操作" width="100"><template #default="{ row }"><el-button size="small" @click="reply(row)">处理</el-button></template></el-table-column>
    </el-table>
  </div>
</template>
