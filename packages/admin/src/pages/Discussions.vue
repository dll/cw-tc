<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listAdminDiscussions, removeUGC, restoreUGC, type AdminDiscussionItem } from '@/api/client'

const rows = ref<AdminDiscussionItem[]>([])
const loading = ref(false)
const status = ref('visible')

async function load() {
  loading.value = true
  try {
    rows.value = await listAdminDiscussions(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '讨论加载失败')
  } finally {
    loading.value = false
  }
}

async function remove(row: AdminDiscussionItem) {
  try {
    const { value } = await ElMessageBox.prompt('下架必须选择违规原因：advertising / abuse / privacy / illegal / irrelevant / other（无差评选项）', '下架讨论', {
      inputValidator: (v: string) => (v && v.trim() ? true : '原因必填'),
    })
    await removeUGC('topic', row.id, value.trim())
    ElMessage.success('已下架并通知作者')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function restore(row: AdminDiscussionItem) {
  try {
    const { value } = await ElMessageBox.prompt('恢复说明', '恢复讨论', { inputValidator: (v: string) => (v && v.trim() ? true : '说明必填') })
    await restoreUGC('topic', row.id, value.trim())
    ElMessage.success('已恢复')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>讨论治理（管理员可下架 / 恢复；教师无删除权）</template>
    <el-radio-group v-model="status" @change="load" style="margin-bottom: 12px">
      <el-radio-button label="visible">可见</el-radio-button>
      <el-radio-button label="removed">已下架</el-radio-button>
      <el-radio-button label="">全部</el-radio-button>
    </el-radio-group>
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="course_code" label="课程码" width="120" />
      <el-table-column label="讲次" width="80"><template #default="{ row }">第 {{ row.lecture_no }} 讲</template></el-table-column>
      <el-table-column prop="body" label="内容" min-width="260" />
      <el-table-column label="作者" width="160"><template #default="{ row }">{{ row.student_name }}（{{ row.student_no }}）</template></el-table-column>
      <el-table-column prop="reply_count" label="回复" width="80" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="remove_reason_kind" label="下架原因" width="120" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button v-if="row.status === 'visible'" size="small" type="danger" plain @click="remove(row)">下架</el-button>
          <el-button v-else size="small" type="success" @click="restore(row)">恢复</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>