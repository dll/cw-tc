<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAdminCourses,
  reviewCourse,
  publishCourse,
  unpublishCourse,
  setPrice,
  setCourseMajor,
  setCourseFreePreview,
  listMajors,
  listCourseRevisions,
  getCourseQuality,
  setCourseQuality,
  bulkPublishCourses,
  bulkUnpublishCourses,
  type BulkCourseResult,
  type AdminCourse,
  type CourseRevision,
  type CourseQuality,
  type MajorItem,
} from '@/api/client'

const rows = ref<AdminCourse[]>([])
const selected = ref<AdminCourse[]>([])
const tableRef = ref()
const loading = ref(false)
const majors = ref<MajorItem[]>([])
const stats = ref<Record<string, number>>({})
const totalCourses = ref(0)
const totalLectures = ref(0)
const filter = ref({ status: '', source: '', q: '' })
const revisionVisible = ref(false)
const revisionTarget = ref<AdminCourse | null>(null)
const revisions = ref<CourseRevision[]>([])

async function openRevisions(row: AdminCourse) {
  revisionTarget.value = row
  try {
    revisions.value = await listCourseRevisions(row.course_code)
    revisionVisible.value = true
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '版本历史加载失败')
  }
}

const qualityVisible = ref(false)
const qualityLoading = ref(false)
const qualityTarget = ref<AdminCourse | null>(null)
const quality = ref<CourseQuality | null>(null)
const qualityForm = ref<{ quality_score?: number; ai_level: string; reason: string }>({
  quality_score: undefined,
  ai_level: '',
  reason: '',
})

async function openQuality(row: AdminCourse) {
  qualityTarget.value = row
  qualityVisible.value = true
  qualityLoading.value = true
  qualityForm.value = { quality_score: undefined, ai_level: '', reason: '' }
  try {
    quality.value = await getCourseQuality(row.course_code)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '质量信息加载失败')
  } finally {
    qualityLoading.value = false
  }
}

async function submitQuality() {
  const target = qualityTarget.value
  if (!target) return
  if (!qualityForm.value.reason.trim()) {
    ElMessage.warning('覆写必须填写事由')
    return
  }
  if (qualityForm.value.quality_score === undefined && !qualityForm.value.ai_level) {
    ElMessage.warning('质量分与 AI 等级至少填写一项')
    return
  }
  try {
    await ElMessageBox.confirm('覆写质量分 / AI 等级会影响对外展示，确认提交？', '二次确认', { type: 'warning' })
    await setCourseQuality(target.course_code, {
      quality_score: qualityForm.value.quality_score,
      ai_level: qualityForm.value.ai_level,
      reason: qualityForm.value.reason.trim(),
      confirm: true,
    })
    ElMessage.success('已覆写（写入审计）')
    quality.value = await getCourseQuality(target.course_code)
    qualityForm.value = { quality_score: undefined, ai_level: '', reason: '' }
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') {
      ElMessage.error(e.message)
    }
  }
}

const statusLabel: Record<string, string> = {
  proposed: '立项', building: '建设中', submitted: '待审核', rejected: '已驳回',
  approved: '待上架', published: '已上架', unpublished: '已下架',
}
const statusOptions = Object.entries(statusLabel).map(([value, label]) => ({ value, label }))

function statCount(key: string): number {
  return stats.value[key] ?? 0
}

async function load() {
  loading.value = true
  try {
    const result = await listAdminCourses({
      status: filter.value.status || undefined,
      source: filter.value.source || undefined,
      q: filter.value.q.trim() || undefined,
    })
    rows.value = result.items
    stats.value = result.stats
    totalCourses.value = result.total
    totalLectures.value = result.total_lectures
    selected.value = []
    tableRef.value?.clearSelection?.()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '课程列表加载失败')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filter.value = { status: '', source: '', q: '' }
  load()
}

function onSelectionChange(items: AdminCourse[]) {
  selected.value = items
}

function failureText(result: BulkCourseResult): string {
  if (!result.failed.length) return ''
  return '\n失败：\n' + result.failed.map((f) => `${f.course_code}：${f.reason}`).join('\n')
}

async function onBulkPublish() {
  const codes = selected.value.map((row) => row.course_code)
  if (!codes.length) return
  try {
    await ElMessageBox.confirm(`确认批量上架 ${codes.length} 门课程？`, '批量上架', { type: 'warning' })
    const result = await bulkPublishCourses(codes)
    ElMessageBox.alert(`成功 ${result.published ?? 0} 门${failureText(result)}`, '批量上架结果')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function onBulkUnpublish() {
  const codes = selected.value.map((row) => row.course_code)
  if (!codes.length) return
  try {
    const { value } = await ElMessageBox.prompt(
      '批量下架必须填写事由（会写审计）。已购学员仍可学习与测评。',
      '批量下架',
      {
        confirmButtonText: '确认下架',
        cancelButtonText: '取消',
        inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
      },
    )
    await ElMessageBox.confirm(`确认下架 ${codes.length} 门课程？`, '二次确认', { type: 'warning' })
    const result = await bulkUnpublishCourses(codes, value.trim())
    ElMessageBox.alert(`成功 ${result.unpublished ?? 0} 门${failureText(result)}`, '批量下架结果')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function loadMajors() {
  majors.value = await listMajors().catch(() => [])
}
onMounted(() => {
  load()
  loadMajors()
})

// —— 专业分类对话框 ——
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
    ElMessage.success('已更新专业分类')
    majorVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '更新失败')
  }
}

// —— 免费试学讲次对话框 ——
const freeVisible = ref(false)
const freeTarget = ref<AdminCourse | null>(null)
const freeForm = ref(0)

function openFree(row: AdminCourse) {
  freeTarget.value = row
  freeForm.value = row.free_preview_lectures ?? 0
  freeVisible.value = true
}

async function submitFree() {
  const target = freeTarget.value
  if (!target) return
  try {
    await setCourseFreePreview(target.course_code, freeForm.value)
    ElMessage.success('已更新免费试学讲次')
    freeVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '更新失败')
  }
}

async function onPublish(row: AdminCourse) {
  const action = row.status === 'unpublished' ? '重新上架' : '上架'
  try {
    await ElMessageBox.confirm(
      `确认${action}「${row.name}」？上架后学员可见并可购买。`,
      `${action}确认`,
      { type: 'warning' },
    )
    await publishCourse(row.course_code)
    ElMessage.success(`已${action}`)
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') {
      ElMessage.error(e.message)
    }
  }
}

async function onUnpublish(row: AdminCourse) {
  try {
    const { value } = await ElMessageBox.prompt(
      '下架必须填写事由（会写审计）。下架后课程从课程广场隐藏；已购学员仍可学习与测评。',
      `下架「${row.name}」`,
      {
        confirmButtonText: '确认下架',
        cancelButtonText: '取消',
        inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
      },
    )
    await ElMessageBox.confirm('确认下架该课程？此操作会立即对未购买学员隐藏。', '二次确认', {
      type: 'warning',
    })
    await unpublishCourse(row.course_code, value.trim())
    ElMessage.success('已下架')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') {
      ElMessage.error(e.message)
    }
  }
}

async function onPrice(row: AdminCourse) {
  try {
    const { value } = await ElMessageBox.prompt('请输入新价格（分）', `改价 · ${row.name}`, {
      inputValue: String(row.price_cents),
      inputValidator: (v) => (/^\d+$/.test(v) ? true : '请输入非负整数（分）'),
    })
    await setPrice(row.course_code, Number(value), true)
    ElMessage.success('已改价（写入审计）')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') {
      ElMessage.error(e.message)
    }
  }
}

// —— 六维审核对话框 ——
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
    <template #header>课程管理（含草稿 / 待审 / 已下架；上架与下架都会写审计）</template>

    <el-row :gutter="12" style="margin-bottom: 14px">
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">全部课程</div>
          <div class="stat-value">{{ totalCourses }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">已上架</div>
          <div class="stat-value">{{ statCount('published') }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">已下架</div>
          <div class="stat-value">{{ statCount('unpublished') }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">待审核</div>
          <div class="stat-value">{{ statCount('submitted') }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">待上架</div>
          <div class="stat-value">{{ statCount('approved') }}</div>
        </el-card>
      </el-col>
      <el-col :span="4">
        <el-card shadow="never" class="stat-card">
          <div class="stat-label">总讲次</div>
          <div class="stat-value">{{ totalLectures }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-form :inline="true" style="margin-bottom: 10px">
      <el-form-item label="状态">
        <el-select v-model="filter.status" placeholder="全部" clearable style="width: 130px" @change="load">
          <el-option v-for="s in statusOptions" :key="s.value" :label="s.label" :value="s.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="来源">
        <el-select v-model="filter.source" placeholder="全部" clearable style="width: 120px" @change="load">
          <el-option label="上游" value="upstream" />
          <el-option label="平台" value="platform" />
        </el-select>
      </el-form-item>
      <el-form-item label="关键词">
        <el-input v-model="filter.q" placeholder="课程码 / 名称 / 简介" clearable style="width: 240px" @keyup.enter="load" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </el-form-item>
    </el-form>

    <div style="margin-bottom: 10px">
      <el-button type="success" :disabled="!selected.length" @click="onBulkPublish">批量上架</el-button>
      <el-button type="danger" plain :disabled="!selected.length" @click="onBulkUnpublish">批量下架</el-button>
      <span style="margin-left: 8px; color: var(--el-text-color-secondary)">已选 {{ selected.length }} 门</span>
    </div>

    <el-table
      ref="tableRef"
      v-loading="loading"
      :data="rows"
      stripe
      @selection-change="onSelectionChange"
    >
      <el-table-column type="selection" width="50" />
      <el-table-column prop="course_code" label="课程码" width="120" />
      <el-table-column label="课程" min-width="220">
        <template #default="{ row }">
          <div>{{ row.name }}</div>
          <div v-if="row.intro" class="course-intro">{{ row.intro }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="source" label="来源" width="90" />
      <el-table-column label="专业分类" width="130">
        <template #default="{ row }">{{ row.major_name ?? '未分类' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="版本" width="150">
        <template #default="{ row }">
          <el-link type="primary" @click="openRevisions(row)">内容 v{{ row.content_version }}</el-link>
          <div class="course-version">线上 {{ row.published_version ? `v${row.published_version}` : '未发布' }}</div>
          <el-tag v-if="row.has_unpublished_changes" type="warning" size="small">待发布</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="价格" width="90">
        <template #default="{ row }">¥{{ (row.price_cents / 100).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column label="讲次" width="80">
        <template #default="{ row }">{{ row.lecture_count }} 讲</template>
      </el-table-column>
      <el-table-column label="免费讲次" width="90">
        <template #default="{ row }">{{ row.free_preview_lectures ?? 0 }}</template>
      </el-table-column>
      <el-table-column label="更新时间" width="170">
        <template #default="{ row }">{{ row.updated_at || row.created_at }}</template>
      </el-table-column>
      <el-table-column label="操作" width="490">
        <template #default="{ row }">
          <el-button v-if="row.status === 'submitted'" size="small" type="primary" @click="openReview(row)">审核</el-button>
          <el-button
            v-if="row.status === 'approved' || row.status === 'unpublished'"
            size="small"
            type="success"
            @click="onPublish(row)"
          >{{ row.status === 'unpublished' ? '重新上架' : '上架' }}</el-button>
          <el-button v-if="row.status === 'published'" size="small" type="danger" plain @click="onUnpublish(row)">下架</el-button>
          <el-button v-if="row.status === 'published'" size="small" @click="onPrice(row)">改价</el-button>
          <el-button size="small" @click="openMajor(row)">专业</el-button>
          <el-button size="small" @click="openFree(row)">免费讲次</el-button>
          <el-button size="small" type="info" plain @click="openQuality(row)">质量/AI</el-button>
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
          <el-form-item label="内容与结构(0-25)"><el-input-number v-model="reviewForm.score_material" :min="0" :max="25" /></el-form-item>
          <el-form-item label="目标契合(0-15)"><el-input-number v-model="reviewForm.score_objective" :min="0" :max="15" /></el-form-item>
          <el-form-item label="案例贯穿(0-15)"><el-input-number v-model="reviewForm.score_case" :min="0" :max="15" /></el-form-item>
          <el-form-item label="教案(0-15)"><el-input-number v-model="reviewForm.score_lesson_plan" :min="0" :max="15" /></el-form-item>
          <el-form-item label="课件(0-15)"><el-input-number v-model="reviewForm.score_slides" :min="0" :max="15" /></el-form-item>
          <el-form-item label="实验(0-15)"><el-input-number v-model="reviewForm.score_lab" :min="0" :max="15" /></el-form-item>
        </div>
        <el-form-item v-if="reviewForm.verdict === 'rejected'" label="问题清单（必填）">
          <el-input v-model="reviewForm.issue_list" type="textarea" :rows="3" placeholder='[{"dim":"material","issue":"..."}]' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReview">提交审核</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="majorVisible" title="更新专业分类" width="400px">
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

    <el-dialog v-model="freeVisible" title="设置免费试学讲次" width="400px">
      <el-form label-position="top">
        <el-form-item label="免费试学讲次数（前 N 讲对未购学员开放；0 = 关闭）">
          <el-input-number v-model="freeForm" :min="0" :max="999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="freeVisible = false">取消</el-button>
        <el-button type="primary" @click="submitFree">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="revisionVisible" :title="`版本历史 · ${revisionTarget?.name ?? ''}`" width="680px">
      <el-table :data="revisions" size="small">
        <el-table-column prop="revision_no" label="改版" width="80" />
        <el-table-column label="内容版本" width="100">
          <template #default="{ row }">v{{ row.content_version }}</template>
        </el-table-column>
        <el-table-column prop="changed_digest" label="变更摘要" />
        <el-table-column prop="published_at" label="发布时间" width="200" />
      </el-table>
      <el-empty v-if="!revisions.length" description="尚无发布记录" />
    </el-dialog>

    <el-drawer v-model="qualityVisible" title="课程质量与 AI 等级" size="560px">
      <div v-loading="qualityLoading">
        <el-descriptions v-if="quality" :column="2" border>
          <el-descriptions-item label="质量分">{{ quality.quality_score ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="AI 等级">{{ quality.ai_level ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="AI 开关">{{ quality.ai_enabled ? '开启' : '关闭' }}</el-descriptions-item>
          <el-descriptions-item label="知识库">{{ quality.kb_status }}</el-descriptions-item>
          <el-descriptions-item label="内容版本">v{{ quality.content_version }}</el-descriptions-item>
          <el-descriptions-item label="线上版本">{{ quality.published_version ? `v${quality.published_version}` : '未发布' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider>最近一次审核</el-divider>
        <template v-if="quality?.latest_review">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="轮次">{{ quality.latest_review.round }}</el-descriptions-item>
            <el-descriptions-item label="结论">{{ quality.latest_review.verdict === 'approved' ? '通过' : '驳回' }}</el-descriptions-item>
            <el-descriptions-item label="总分">{{ quality.latest_review.total_score }}</el-descriptions-item>
            <el-descriptions-item label="审核版本">v{{ quality.latest_review.content_version }}</el-descriptions-item>
            <el-descriptions-item label="审核人">{{ quality.latest_review.reviewer_name ?? quality.latest_review.reviewer_role }}</el-descriptions-item>
            <el-descriptions-item label="时间">{{ quality.latest_review.reviewed_at }}</el-descriptions-item>
          </el-descriptions>
          <div class="score-grid">
            <span>材料 {{ quality.latest_review.score_material }}/25</span>
            <span>目标 {{ quality.latest_review.score_objective }}/15</span>
            <span>案例 {{ quality.latest_review.score_case }}/15</span>
            <span>教案 {{ quality.latest_review.score_lesson_plan }}/15</span>
            <span>课件 {{ quality.latest_review.score_slides }}/15</span>
            <span>实验 {{ quality.latest_review.score_lab }}/15</span>
          </div>
          <el-alert
            v-if="quality.latest_review.issue_list"
            :title="quality.latest_review.issue_list"
            type="warning"
            :closable="false"
            style="margin-top: 8px"
          />
        </template>
        <el-empty v-else description="尚无审核记录" />

        <el-divider>核定覆写（写审计）</el-divider>
        <el-form label-position="top">
          <el-form-item label="质量分（0-100，留空不改）">
            <el-input-number v-model="qualityForm.quality_score" :min="0" :max="100" :precision="1" />
          </el-form-item>
          <el-form-item label="AI 等级（留空不改）">
            <el-select v-model="qualityForm.ai_level" clearable placeholder="不修改" style="width: 100%">
              <el-option label="LA" value="LA" />
              <el-option label="LB" value="LB" />
              <el-option label="LC" value="LC" />
            </el-select>
          </el-form-item>
          <el-form-item label="事由（必填）">
            <el-input v-model="qualityForm.reason" type="textarea" :rows="2" />
          </el-form-item>
          <el-button type="primary" @click="submitQuality">保存覆写</el-button>
        </el-form>
      </div>
    </el-drawer>
  </el-card>
</template>

<style scoped>
.stat-card {
  text-align: center;
}
.stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.stat-value {
  font-size: 24px;
  font-weight: 600;
  margin-top: 4px;
}
.course-intro {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.course-version {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.score-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
