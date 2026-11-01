<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listTeacherDiscussions,
  replyTeacherDiscussion,
  type TeacherDiscussionItem,
  type TeacherDiscussionSummary,
} from '@/api/client'

const rows = ref<TeacherDiscussionItem[]>([])
const summary = ref<TeacherDiscussionSummary[]>([])
const loading = ref(false)
const courseCode = ref('')
const onlyUnanswered = ref(false)

async function load() {
  loading.value = true
  try {
    const result = await listTeacherDiscussions({
      course_code: courseCode.value.trim() || undefined,
      only_unanswered: onlyUnanswered.value ? 'true' : undefined,
    })
    rows.value = result.items
    summary.value = result.summary
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '讨论加载失败')
  } finally {
    loading.value = false
  }
}

async function reply(row: TeacherDiscussionItem) {
  try {
    const { value } = await ElMessageBox.prompt('教师回复会显示“官方”标识并通知发帖学员；不得发布测评题目或答案。', '回复讨论', {
      confirmButtonText: '发布回复',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim().length >= 2 ? true : '回复至少 2 个字'),
    })
    await replyTeacherDiscussion(row.id, value.trim())
    ElMessage.success('已回复并通知学员')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>讨论与问答（只看自己的课程 · 未回复优先）</template>
    <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 12px; flex-wrap: wrap">
      <el-input v-model="courseCode" placeholder="课程码（可选）" clearable style="width: 200px" @keyup.enter="load" />
      <el-switch v-model="onlyUnanswered" active-text="只看未回复" @change="load" />
      <el-button type="primary" @click="load">查询</el-button>
    </div>

    <el-table :data="summary" size="small" style="margin-bottom: 12px">
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="course_name" label="课程" min-width="180" />
      <el-table-column prop="topics" label="讨论数" width="100" />
      <el-table-column label="未回复" width="100">
        <template #default="{ row }">
          <el-tag :type="row.unanswered > 0 ? 'warning' : 'success'" size="small">{{ row.unanswered }}</el-tag>
        </template>
      </el-table-column>
    </el-table>

    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="course_code" label="课程码" width="120" />
      <el-table-column label="讲次" width="90">
        <template #default="{ row }">第 {{ row.lecture_no }} 讲</template>
      </el-table-column>
      <el-table-column prop="body" label="讨论内容" min-width="260" />
      <el-table-column label="学员" width="150">
        <template #default="{ row }">{{ row.student_name ?? '-' }}<span v-if="row.student_no">（{{ row.student_no }}）</span></template>
      </el-table-column>
      <el-table-column prop="reply_count" label="回复" width="80" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.unanswered ? 'warning' : 'success'" size="small">{{ row.unanswered ? '未回复' : '已回复' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="last_activity" label="最近活动" width="180" />
      <el-table-column label="操作" width="110">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="reply(row)">官方回复</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
