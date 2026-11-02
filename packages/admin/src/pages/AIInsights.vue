<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { generateAIInsight, getAIInsight, listAIInsights, type AIInsightItem } from '@/api/client'

const courseCode = ref('')
const rows = ref<AIInsightItem[]>([])
const loading = ref(false)
const detail = ref<Record<string, unknown> | null>(null)

async function load() {
  if (!courseCode.value.trim()) return
  loading.value = true
  try {
    rows.value = await listAIInsights(courseCode.value.trim())
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '报告加载失败')
  } finally {
    loading.value = false
  }
}

async function generate() {
  if (!courseCode.value.trim()) return
  try {
    await ElMessageBox.confirm('将基于 T+1 统计、评价、讨论与异常数据生成 AI 报告；每条建议必须可溯源。确认生成？', 'AI 运营报告', { type: 'warning' })
    const result = await generateAIInsight(courseCode.value.trim())
    detail.value = result
    ElMessage.success('已生成')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function open(row: AIInsightItem) {
  try {
    detail.value = await getAIInsight(courseCode.value.trim(), row.id)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '报告详情读取失败')
  }
}
</script>

<template>
  <el-card>
    <template #header>AI 运营报告（每条建议必须可溯源）</template>
    <div style="display: flex; gap: 12px; margin-bottom: 12px">
      <el-input v-model="courseCode" placeholder="课程码" style="width: 220px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button type="success" @click="generate">生成报告</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="period_start" label="周期开始" width="130" />
      <el-table-column prop="period_end" label="周期结束" width="130" />
      <el-table-column prop="summary" label="摘要" min-width="260" />
      <el-table-column prop="generated_at" label="生成时间" width="180" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }"><el-button size="small" @click="open(row)">详情</el-button></template>
      </el-table-column>
    </el-table>
    <el-drawer :model-value="detail !== null" title="AI 运营报告" size="60%" @close="detail = null">
      <pre style="white-space: pre-wrap">{{ JSON.stringify(detail, null, 2) }}</pre>
    </el-drawer>
  </el-card>
</template>