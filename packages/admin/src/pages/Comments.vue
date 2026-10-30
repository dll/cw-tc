<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listAdminComments, removeAdminComment, type AdminComment } from '@/api/client'

const rows = ref<AdminComment[]>([])
const loading = ref(false)
const status = ref('visible')

async function load() {
  loading.value = true
  try {
    rows.value = await listAdminComments(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '评价加载失败')
  } finally {
    loading.value = false
  }
}

async function remove(row: AdminComment) {
  try {
    const { value } = await ElMessageBox.prompt('请填写违规原因类别（不含“差评”选项）', '下架评价', {
      inputPattern: /\S+/,
      inputErrorMessage: '原因不能为空',
    })
    await removeAdminComment(row.id, value)
    ElMessage.success('评价已下架')
    await load()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    ElMessage.error(e instanceof Error ? e.message : '下架失败')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <el-radio-group v-model="status" @change="load">
        <el-radio-button label="visible">可见</el-radio-button>
        <el-radio-button label="removed">已下架</el-radio-button>
        <el-radio-button label="">全部</el-radio-button>
      </el-radio-group>
    </div>
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="course_name" label="课程" width="180" />
      <el-table-column prop="nickname" label="昵称" width="110" />
      <el-table-column prop="rating" label="评分" width="80" />
      <el-table-column prop="body" label="内容" />
      <el-table-column prop="pending_reports" label="举报" width="80" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button v-if="row.status !== 'removed'" size="small" type="danger" @click="remove(row)">下架</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
