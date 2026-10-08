<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { listMajors, createMajor, updateMajor, type MajorItem } from '@/api/client'

const rows = ref<MajorItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await listMajors()
  } finally {
    loading.value = false
  }
}
onMounted(load)

const name = ref('')
async function onCreate() {
  if (!name.value.trim()) {
    ElMessage.warning('请输入专业名')
    return
  }
  try {
    await createMajor(name.value.trim(), rows.value.length + 1)
    name.value = ''
    ElMessage.success('已创建')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '创建失败')
  }
}

async function onToggle(row: MajorItem) {
  try {
    await updateMajor(row.id, { status: row.status === 'enabled' ? 'disabled' : 'enabled' })
    ElMessage.success('已更新')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '更新失败')
  }
}
</script>

<template>
  <el-card>
    <template #header>专业分类（平台自有数据，不来自上游）</template>
    <div style="margin-bottom: 12px">
      <el-input v-model="name" placeholder="新专业名" style="width: 240px; margin-right: 8px" />
      <el-button type="primary" @click="onCreate">新建</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="name" label="名称" />
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button size="small" :type="row.status === 'enabled' ? 'danger' : 'success'" @click="onToggle(row)">
            {{ row.status === 'enabled' ? '停用' : '启用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
