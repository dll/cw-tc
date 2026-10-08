<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAdminCourses,
  reviewCourse,
  publishCourse,
  setPrice,
  setCourseMajor,
  listMajors,
  type AdminCourse,
  type MajorItem,
} from '@/api/client'

const rows = ref<AdminCourse[]>([])
const loading = ref(false)
const majors = ref<MajorItem[]>([])

const statusLabel: Record<string, string> = {
  proposed: '立项', building: '建设中', submitted: '待审', rejected: '已驳回',
  approved: '待上架', published: '已上架', unpublished: '已下架',
}

async function load() {
  loading.value = true
  try {
    rows.value = await listAdminCourses()
  } finally {
    loading.value = false
  }
}
async function loadMajors() {
  majors.value = await listMajors().catch(() => [])
}
onMounted(() => {
  load()
  loadMajors()
})

// 设置专业分类对话框
const majorVisible = ref(false)
const majorTarget = ref<AdminCourse | null>(null)
const majorForm = ref<number | null>(null)

function openMajor(row: AdminCourse) {
  majorTarget.value = row
  majorForm.value = row.major_id
  majorVisible.value = true
}

async function submitMajor() {
  const target = majorTarget.value
  if (!target || majorForm.value == null) return
  try {
    await setCourseMajor(target.course_code, majorForm.value)
    ElMessage.success('已设置专业分类')
    majorVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '设置失败')
  }
}

async function onPublish(row: AdminCourse) {
  await ElMessageBox.confirm(`确认上架「${row.name}」？`, '上架确认', { type: 'warning' })
  try {
    await publishCourse(row.course_code)
    ElMessage.success('已上架')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '上架失败')
  }
}

async function onPrice(row: AdminCourse) {
  const { value } = await ElMessageBox.prompt('输入新价格（分）', `改价 · ${row.name}`, {
    inputValue: String(row.price_cents),
    inputValidator: (v) => (/^\d+$/.test(v) ? true : '请输入非负整数（分）'),
  })
  try {
    await setPrice(row.course_code, Number(value), true)
    ElMessage.success('已改价（写审计）')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '改价失败')
  }
}

const reviewVisible = ref(false)
const reviewTarget = ref<AdminCourse | null>(null)
const reviewForm = ref({
  verdict: 'approved',
  score_material: 25, score_objective: 15, score_case: 15,
  score_lesson_plan: 15, score_slides: 15, score_lab: 15,
  issue_list: '',
})

function openReview(row: AdminCourse) {
  reviewTarget.value = row
  reviewForm.value = {
    verdict: 'approved', score_material: 25, score_objective: 15, score_case: 15,
    score_lesson_plan: 15, score_slides: 15, score_lab: 15, issue_list: '',
  }
  reviewVisible.value = true
}

async function submitReview() {
  const target = reviewTarget.value
  if (!target) return
  const payload: Record<string, unknown> = {
    verdict: reviewForm.value.verdict,
    content_version: 1,
    score_material: reviewForm.value.score_material,
    score_objective: reviewForm.value.score_objective,
    score_case: reviewForm.value.score_case,
    score_lesson_plan: reviewForm.value.score_lesson_plan,
    score_slides: reviewForm.value.score_slides,
    score_lab: reviewForm.value.score_lab,
    issue_list: reviewForm.value.issue_list,
  }
  try {
    await reviewCourse(target.course_code, payload)
    ElMessage.success('审核已提交')
    reviewVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '审核失败')
  }
}
</script>

<template>
  <el-card>
    <template #header>课程管理（含未上架/待审）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="course_code" label="课程码" width="120" />
      <el-table-column prop="name" label="课程名" />
      <el-table-column prop="source" label="来源" width="90" />
      <el-table-column label="专业分类" width="140">
        <template #default="{ row }">{{ row.major_name ?? '未分类' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="价格" width="90">
        <template #default="{ row }">¥{{ (row.price_cents / 100).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="300">
        <template #default="{ row }">
          <el-button v-if="row.status === 'submitted'" size="small" type="primary" @click="openReview(row)">审核</el-button>
          <el-button v-if="row.status === 'approved'" size="small" type="success" @click="onPublish(row)">上架</el-button>
          <el-button v-if="row.status === 'published'" size="small" @click="onPrice(row)">改价</el-button>
          <el-button size="small" @click="openMajor(row)">设置专业</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="reviewVisible" title="六维审核" width="560px">
      <el-form label-position="top">
        <el-form-item label="结论">
          <el-radio-group v-model="reviewForm.verdict">
            <el-radio value="approved">通过</el-radio>
            <el-radio value="rejected">驳回</el-radio>
          </el-radio-group>
        </el-form-item>
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px">
          <el-form-item label="材料完整性(0-25)"><el-input-number v-model="reviewForm.score_material" :min="0" :max="25" /></el-form-item>
          <el-form-item label="目标合规(0-15)"><el-input-number v-model="reviewForm.score_objective" :min="0" :max="15" /></el-form-item>
          <el-form-item label="案例贯穿(0-15)"><el-input-number v-model="reviewForm.score_case" :min="0" :max="15" /></el-form-item>
          <el-form-item label="教案(0-15)"><el-input-number v-model="reviewForm.score_lesson_plan" :min="0" :max="15" /></el-form-item>
          <el-form-item label="课件(0-15)"><el-input-number v-model="reviewForm.score_slides" :min="0" :max="15" /></el-form-item>
          <el-form-item label="实验(0-15)"><el-input-number v-model="reviewForm.score_lab" :min="0" :max="15" /></el-form-item>
        </div>
        <el-form-item v-if="reviewForm.verdict === 'rejected'" label="问题清单（驳回必填）">
          <el-input v-model="reviewForm.issue_list" type="textarea" :rows="3" placeholder='[{"dim":"material","issue":"..."}]' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReview">提交审核</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="majorVisible" title="设置专业分类" width="400px">
      <el-form label-position="top">
        <el-form-item label="专业分类">
          <el-select v-model="majorForm" placeholder="请选择专业分类" style="width: 100%">
            <el-option v-for="m in majors" :key="m.id" :label="m.name" :value="m.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="majorVisible = false">取消</el-button>
        <el-button type="primary" @click="submitMajor">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>
