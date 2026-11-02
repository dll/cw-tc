<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAdminShowcase,
  listJobMap,
  listWebContent,
  updateAdminShowcase,
  updateJobMap,
  updateWebContent,
  type JobMapItem,
  type ShowcaseAdminItem,
  type WebContentItem,
} from '@/api/client'

const tab = ref('content')
const content = ref<WebContentItem[]>([])
const jobMap = ref<JobMapItem[]>([])
const showcase = ref<ShowcaseAdminItem[]>([])
const contentVisible = ref(false)
const contentForm = ref({ block: 'highlight', slot: 1, title: '', body: '', evidence_ref: '', status: 'draft', confirm: false })
const jobVisible = ref(false)
const jobForm = ref({ id: 0, course_code: '', cert_level: 'L1', job_direction: '', skill_items: '', lecture_refs: '', evidence_ref: '', status: 'draft', major_id: 0, confirm: false })

async function load() {
  try {
    content.value = await listWebContent('')
    jobMap.value = await listJobMap('')
    showcase.value = await listAdminShowcase('')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '对外内容加载失败')
  }
}

function openContent(row?: WebContentItem) {
  contentForm.value = row
    ? { block: row.block, slot: row.slot, title: row.title, body: row.body, evidence_ref: row.evidence_ref, status: row.status, confirm: false }
    : { block: 'highlight', slot: 1, title: '', body: '', evidence_ref: '', status: 'draft', confirm: false }
  contentVisible.value = true
}

async function submitContent() {
  try {
    await ElMessageBox.confirm('发布 / 下线对外内容会立即影响官网，确认继续？', '对外内容确认', { type: 'warning' })
    await updateWebContent({ ...contentForm.value, confirm: true })
    contentVisible.value = false
    ElMessage.success('已保存并生成新版本')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

function openJob(row?: JobMapItem) {
  jobForm.value = row
    ? { id: row.id, course_code: row.course_code, cert_level: row.cert_level, job_direction: row.job_direction,
        skill_items: JSON.stringify(row.skill_items), lecture_refs: JSON.stringify(row.lecture_refs),
        evidence_ref: row.evidence_ref, status: row.status, major_id: row.major_id, confirm: false }
    : { id: 0, course_code: '', cert_level: 'L1', job_direction: '', skill_items: '[]', lecture_refs: '[]', evidence_ref: '', status: 'draft', major_id: 0, confirm: false }
  jobVisible.value = true
}

async function submitJob() {
  try {
    const skillItems = JSON.parse(jobForm.value.skill_items || '[]')
    const lectureRefs = JSON.parse(jobForm.value.lecture_refs || '[]')
    await ElMessageBox.confirm('发布就业映射必须同时有依据与讲次引用，确认？', '就业映射确认', { type: 'warning' })
    await updateJobMap({ ...jobForm.value, skill_items: skillItems, lecture_refs: lectureRefs, confirm: true })
    jobVisible.value = false
    ElMessage.success('已保存')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function verifyShowcase(row: ShowcaseAdminItem, action: 'verify' | 'withdraw') {
  try {
    const { value } = await ElMessageBox.prompt(action === 'verify' ? '核验说明（只核验不存档）' : '下架原因', action === 'verify' ? '核验成果' : '下架成果', {
      inputValidator: (v: string) => (v && v.trim() ? true : '请填写说明'),
    })
    await updateAdminShowcase(row.id, action, value.trim())
    ElMessage.success('已处理')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>对外展示（官网内容 / 就业映射 / 学员成果）</template>
    <el-tabs v-model="tab">
      <el-tab-pane label="内容块" name="content">
        <el-button type="primary" @click="openContent()" style="margin-bottom: 12px">新增 / 发布内容</el-button>
        <el-table :data="content" size="small">
          <el-table-column prop="block" label="区块" width="140" />
          <el-table-column prop="slot" label="位置" width="70" />
          <el-table-column prop="title" label="标题" width="160" />
          <el-table-column prop="body" label="正文" min-width="260" />
          <el-table-column prop="evidence_ref" label="依据" min-width="160" />
          <el-table-column prop="version" label="版本" width="70" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column label="操作" width="90"><template #default="{ row }"><el-button size="small" @click="openContent(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="就业映射" name="job">
        <el-button type="primary" @click="openJob()" style="margin-bottom: 12px">新增 / 发布映射</el-button>
        <el-table :data="jobMap" size="small">
          <el-table-column prop="course_code" label="课程码" width="130" />
          <el-table-column prop="cert_level" label="等级" width="70" />
          <el-table-column prop="job_direction" label="岗位方向" min-width="180" />
          <el-table-column prop="evidence_ref" label="依据" min-width="160" />
          <el-table-column prop="status" label="状态" width="100" />
          <el-table-column label="操作" width="90"><template #default="{ row }"><el-button size="small" @click="openJob(row)">编辑</el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
      <el-tab-pane label="学员成果" name="showcase">
        <el-table :data="showcase" size="small">
          <el-table-column prop="nickname" label="昵称" width="120" />
          <el-table-column prop="kind" label="类型" width="120" />
          <el-table-column prop="summary" label="摘要" min-width="220" />
          <el-table-column prop="status" label="状态" width="110" />
          <el-table-column prop="display_consent_at" label="授权时间" width="180" />
          <el-table-column label="操作" width="180">
            <template #default="{ row }">
              <el-button size="small" type="success" @click="verifyShowcase(row, 'verify')">核验</el-button>
              <el-button size="small" type="danger" plain @click="verifyShowcase(row, 'withdraw')">下架</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="contentVisible" title="对外内容块" width="620px">
      <el-form label-position="top">
        <el-form-item label="区块"><el-input v-model="contentForm.block" /></el-form-item>
        <el-form-item label="位置（slot）"><el-input-number v-model="contentForm.slot" :min="0" /></el-form-item>
        <el-form-item label="标题"><el-input v-model="contentForm.title" /></el-form-item>
        <el-form-item label="正文"><el-input v-model="contentForm.body" type="textarea" :rows="4" /></el-form-item>
        <el-form-item label="可核对依据"><el-input v-model="contentForm.evidence_ref" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="contentForm.status"><el-option label="草稿" value="draft" /><el-option label="发布" value="published" /><el-option label="下线" value="retired" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="contentVisible = false">取消</el-button><el-button type="primary" @click="submitContent">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="jobVisible" title="就业直达映射" width="620px">
      <el-form label-position="top">
        <el-form-item label="课程码"><el-input v-model="jobForm.course_code" /></el-form-item>
        <el-form-item label="证书等级"><el-select v-model="jobForm.cert_level"><el-option label="L1" value="L1" /><el-option label="L2" value="L2" /><el-option label="L3" value="L3" /></el-select></el-form-item>
        <el-form-item label="岗位方向"><el-input v-model="jobForm.job_direction" /></el-form-item>
        <el-form-item label="能力项（JSON 数组）"><el-input v-model="jobForm.skill_items" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="讲次引用（JSON 数组）"><el-input v-model="jobForm.lecture_refs" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="依据"><el-input v-model="jobForm.evidence_ref" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="jobForm.status"><el-option label="草稿" value="draft" /><el-option label="发布" value="published" /><el-option label="下线" value="retired" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="jobVisible = false">取消</el-button><el-button type="primary" @click="submitJob">保存</el-button></template>
    </el-dialog>
  </el-card>
</template>