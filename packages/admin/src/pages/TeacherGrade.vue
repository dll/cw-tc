<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  listGradeReviews,
  gradeDetail,
  submitGrade,
  type GradeListItem,
  type GradeQuestion,
} from '@/api/client'

const rows = ref<GradeListItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await listGradeReviews()
  } finally {
    loading.value = false
  }
}
onMounted(load)

const dialogVisible = ref(false)
const detailLoading = ref(false)
const current = ref<GradeListItem | null>(null)
const questions = ref<GradeQuestion[]>([])
const scores = ref<Record<number, number>>({})
const comments = ref<Record<number, string>>({})

async function openDetail(row: GradeListItem) {
  current.value = row
  dialogVisible.value = true
  detailLoading.value = true
  scores.value = {}
  comments.value = {}
  try {
    const d = await gradeDetail(row.ResultID)
    questions.value = d.items
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '加载失败')
  } finally {
    detailLoading.value = false
  }
}

async function onGrade(q: GradeQuestion) {
  const sc = scores.value[q.question_id]
  if (sc === undefined || sc < 0) {
    ElMessage.warning('请输入分数')
    return
  }
  try {
    await submitGrade(current.value!.ResultID, q.question_id, {
      score: sc,
      comment: comments.value[q.question_id] || '',
    })
    ElMessage.success(`已评阅 ${q.qno}`)
    q.gradability = 'graded'
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '评阅失败')
  }
}
</script>

<template>
  <el-card>
    <template #header>阅卷台（主观题采点制 · 仅自有课程）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="ResultID" label="结果ID" width="80" />
      <el-table-column prop="CourseName" label="课程" />
      <el-table-column prop="Level" label="档位" width="70" />
      <el-table-column prop="AutoScore" label="客观分" width="90" />
      <el-table-column prop="ManualScore" label="主观分" width="90" />
      <el-table-column prop="TotalScore" label="总分" width="90" />
      <el-table-column label="操作" width="110">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="openDetail(row)">阅卷</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="逐题评阅" width="720px">
      <div v-loading="detailLoading">
        <div v-for="q in questions" :key="q.question_id" class="q-block">
          <div class="q-stem">{{ q.qno }} · {{ q.stem }}</div>
          <div class="q-answer">学员作答：{{ q.answer || '（未作答）' }}</div>
          <div v-if="q.gradability !== 'auto'" class="q-grade">
            <span style="margin-right: 8px">满分 {{ q.score }}</span>
            <el-input-number v-model="scores[q.question_id]" :min="0" :max="q.score" size="small" />
            <el-input v-model="comments[q.question_id]" placeholder="评语" size="small" style="width: 200px; margin-left: 8px" />
            <el-button size="small" type="primary" :disabled="q.gradability === 'graded'" style="margin-left: 8px" @click="onGrade(q)">
              {{ q.gradability === 'graded' ? '已评' : '评分' }}
            </el-button>
          </div>
          <div v-else class="q-auto">客观题 · 机器判分</div>
        </div>
      </div>
    </el-dialog>
  </el-card>
</template>

<style scoped>
.q-block { border-bottom: 1px solid var(--el-border-color-light); padding: 10px 0; }
.q-stem { font-weight: 600; }
.q-answer { color: var(--el-text-color-secondary); margin: 6px 0; }
.q-grade { display: flex; align-items: center; }
.q-auto { color: var(--el-text-color-placeholder); font-size: 13px; }
</style>
