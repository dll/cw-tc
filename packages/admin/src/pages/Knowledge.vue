<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listKnowledgeStatus, rebuildKnowledge, type KnowledgeStatusItem } from '@/api/client'

const rows = ref<KnowledgeStatusItem[]>([])
const loading = ref(false)
const totalChunks = ref(0)
const totalAssets = ref(0)
const courseCode = ref('')

async function load() {
  loading.value = true
  try {
    const result = await listKnowledgeStatus(courseCode.value.trim())
    rows.value = result.items
    totalChunks.value = result.total_chunks
    totalAssets.value = result.total_assets
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '知识库状态读取失败')
  } finally {
    loading.value = false
  }
}

async function rebuild(row: KnowledgeStatusItem) {
  try {
    await ElMessageBox.confirm(`重建「${row.course_name}」知识库？只读课程资源，整课替换切片，可重复执行。`, '重建知识库', { type: 'warning' })
    const result = await rebuildKnowledge(row.course_code)
    ElMessage.success(`已重建：${result.assets} 个资源 / ${result.chunks} 个切片`)
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>课程知识库（切片数 / 资源数 / 最后构建时间）</template>
    <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 12px">
      <el-input v-model="courseCode" placeholder="课程码（可选）" clearable style="width: 220px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <span style="color: var(--el-text-color-secondary)">共 {{ totalChunks }} 个切片 / {{ totalAssets }} 个资源</span>
    </div>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="course_code" label="课程码" width="140" />
      <el-table-column prop="course_name" label="课程" min-width="200" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.kb_status === 'ready' ? 'success' : row.kb_status === 'failed' ? 'danger' : 'info'" size="small">{{ row.kb_status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="chunks" label="切片" width="90" />
      <el-table-column prop="assets" label="资源" width="90" />
      <el-table-column prop="last_built" label="最后构建" width="200">
        <template #default="{ row }">{{ row.last_built ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="130">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="rebuild(row)">重建</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>