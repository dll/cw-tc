<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createTeacherAIDraft, listTeacherAIDrafts, reviewTeacherAIDraft, type AIDraftItem } from '@/api/client'

const rows = ref<AIDraftItem[]>([])
const loading = ref(false)
const status = ref('')
const createVisible = ref(false)
const form = ref({ course_code: '', draft_scope: 'single', lecture_count: 1, source_topic_ids: '' })

async function load() {
  loading.value = true
  try {
    rows.value = await listTeacherAIDrafts(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '草稿加载失败')
  } finally {
    loading.value = false
  }
}

async function submitCreate() {
  const ids = form.value.source_topic_ids.split(',').map((v) => Number(v.trim())).filter((v) => v > 0)
  if (!form.value.course_code.trim() || !ids.length) {
    ElMessage.warning('课程码与建议原帖 ID 必填')
    return
  }
  try {
    const draft = await createTeacherAIDraft({
      course_code: form.value.course_code.trim(),
      draft_scope: form.value.draft_scope,
      lecture_count: form.value.draft_scope === 'chapter' ? form.value.lecture_count : 1,
      source_topic_ids: ids,
      kind: 'new_lecture',
      trigger_kind: 'teacher_mark',
    })
    createVisible.value = false
    if (!draft.auto_check_passed) ElMessage.warning('已生成，但自动体检未通过：请查看理由后再修改')
    else ElMessage.success('已生成并通过自动体检，待教师一审')
    await load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '生成失败')
  }
}

async function review(row: AIDraftItem, verdict: 'approved' | 'rejected') {
  try {
    let issues: string[] = []
    if (verdict === 'rejected') {
      const { value } = await ElMessageBox.prompt('驳回必须填写问题清单（每行一条）', '一审驳回', {
        inputType: 'textarea', inputValidator: (v: string) => (v && v.trim() ? true : '问题清单必填'),
      })
      issues = value.split('\n').map((v) => v.trim()).filter(Boolean)
    } else {
      await ElMessageBox.confirm('确认一审通过？通过后将进入管理员二审。', '一审通过', { type: 'warning' })
    }
    await reviewTeacherAIDraft(row.id, verdict, issues)
    ElMessage.success('一审已提交')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>AI 内容草稿（教师一审 · 通过后进管理员二审）</template>
    <div style="display: flex; gap: 12px; margin-bottom: 12px">
      <el-select v-model="status" placeholder="状态" clearable style="width: 180px" @change="load">
        <el-option label="全部状态" value="" />
        <el-option label="待一审" value="pending_teacher" />
        <el-option label="待二审" value="pending_admin" />
        <el-option label="已上架" value="published" />
      </el-select>
      <el-button type="primary" @click="createVisible = true">生成草稿</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column type="expand">
        <template #default="{ row }">
          <pre style="white-space: pre-wrap; max-height: 360px; overflow: auto">{{ row.content_md }}</pre>
          <p style="color: var(--el-text-color-secondary)">自动体检：{{ JSON.stringify(row.auto_check ?? {}) }}</p>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="draft_scope" label="粒度" width="100" />
      <el-table-column prop="lecture_count" label="讲数" width="80" />
      <el-table-column label="体检" width="100">
        <template #default="{ row }">
          <el-tag :type="row.auto_check_passed ? 'success' : 'danger'" size="small">{{ row.auto_check_passed ? '通过' : '未过' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="150" />
      <el-table-column prop="model_version" label="模型版本" width="150" />
      <el-table-column prop="filing_no" label="备案号" width="140" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <template v-if="row.status === 'pending_teacher'">
            <el-button size="small" type="success" @click="review(row, 'approved')">通过</el-button>
            <el-button size="small" type="danger" plain @click="review(row, 'rejected')">驳回</el-button>
          </template>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="生成 AI 内容草稿" width="560px">
      <el-form label-position="top">
        <el-form-item label="课程码"><el-input v-model="form.course_code" /></el-form-item>
        <el-form-item label="建议原帖 ID（逗号分隔，必填）"><el-input v-model="form.source_topic_ids" /></el-form-item>
        <el-form-item label="粒度">
          <el-select v-model="form.draft_scope">
            <el-option label="单条" value="single" />
            <el-option label="章（多讲）" value="chapter" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.draft_scope === 'chapter'" label="讲数（1~5）">
          <el-input-number v-model="form.lecture_count" :min="1" :max="5" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">生成</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>