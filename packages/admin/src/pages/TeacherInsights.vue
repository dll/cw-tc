<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createRevisionPlan,
  getRevisionPlanEffect,
  listRevisionPlans,
  listTeacherInsights,
  updateRevisionPlan,
  verdictTeacherInsight,
  type RevisionPlanItem,
  type TeacherInsightItem,
} from '@/api/client'

const courseCode = ref('')
const insights = ref<TeacherInsightItem[]>([])
const plans = ref<RevisionPlanItem[]>([])
const loading = ref(false)
const planVisible = ref(false)
const planForm = ref({ course_code: '', insight_id: 0, problem: '', evidence: '', measure: '', target_metric: '', priority: 'P1' })

async function load() {
  loading.value = true
  try {
    insights.value = await listTeacherInsights(courseCode.value.trim())
    plans.value = await listRevisionPlans(courseCode.value.trim())
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '数据加载失败')
  } finally {
    loading.value = false
  }
}

async function verdict(row: TeacherInsightItem, accepted: boolean) {
  try {
    const { value } = await ElMessageBox.prompt(accepted ? '采纳意见（可留空）' : '驳回必须填写理由', accepted ? '采纳建议' : '驳回建议', {
      inputType: 'textarea',
      inputValidator: (v: string) => (accepted || (v && v.trim()) ? true : '理由必填'),
    })
    await verdictTeacherInsight(row.id, accepted ? 'accepted' : 'rejected', (value ?? '').trim())
    ElMessage.success('已记录结论')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

function openPlan(row?: TeacherInsightItem) {
  planForm.value = {
    course_code: row?.course_code ?? courseCode.value.trim(),
    insight_id: row?.id ?? 0,
    problem: '', evidence: row ? `AI 周报 #${row.id}` : '', measure: '', target_metric: '', priority: 'P1',
  }
  planVisible.value = true
}

async function submitPlan() {
  try {
    await createRevisionPlan({ ...planForm.value, source: planForm.value.insight_id ? 'ai' : 'manual' })
    planVisible.value = false
    ElMessage.success('改版计划已创建')
    await load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '创建失败')
  }
}

async function planEffect(row: RevisionPlanItem) {
  try {
    const data = await getRevisionPlanEffect(row.id)
    ElMessageBox.alert(JSON.stringify(data, null, 2), '改版效果对比', { confirmButtonText: '关闭' })
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '效果读取失败')
  }
}

async function setPlan(row: RevisionPlanItem, status: string) {
  try {
    let reason = ''
    if (status === 'done') {
      const { value } = await ElMessageBox.prompt('完成改版必须关联版本记录 course_revision ID', '完成改版', {
        inputValidator: (v: string) => (Number(v) > 0 ? true : '请输入有效版本 ID'),
      })
      await updateRevisionPlan(row.id, status, '', Number(value))
      ElMessage.success('已完成并关联版本')
      await load()
      return
    }
    if (status === 'abandoned') {
      const { value } = await ElMessageBox.prompt('放弃改版必须填写原因', '放弃改版', { inputValidator: (v: string) => (v && v.trim() ? true : '原因必填') })
      reason = value.trim()
    }
    await updateRevisionPlan(row.id, status, reason)
    ElMessage.success('已更新')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>AI 周报与改版计划（可溯源 · 教师结论留痕）</template>
    <div style="display: flex; gap: 12px; margin-bottom: 12px">
      <el-input v-model="courseCode" placeholder="课程码（可选）" clearable style="width: 220px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
      <el-button @click="openPlan()">新建改版计划</el-button>
    </div>

    <h4>AI 周报</h4>
    <el-table v-loading="loading" :data="insights" size="small" style="margin-bottom: 16px">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="summary" label="摘要" min-width="240" />
      <el-table-column prop="verdict" label="结论" width="110" />
      <el-table-column prop="verdict_note" label="意见" min-width="160" />
      <el-table-column prop="generated_at" label="生成时间" width="180" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <template v-if="row.verdict === 'pending'">
            <el-button size="small" type="success" @click="verdict(row, true)">采纳</el-button>
            <el-button size="small" type="danger" plain @click="verdict(row, false)">驳回</el-button>
            <el-button size="small" @click="openPlan(row)">转改版计划</el-button>
          </template>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>

    <h4>改版计划</h4>
    <el-table :data="plans" size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="problem" label="问题" min-width="180" />
      <el-table-column prop="evidence" label="依据" min-width="160" />
      <el-table-column prop="measure" label="措施" min-width="180" />
      <el-table-column prop="target_metric" label="目标指标" min-width="160" />
      <el-table-column prop="status" label="状态" width="110" />
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button size="small" @click="setPlan(row, 'in_progress')">进行中</el-button>
          <el-button size="small" type="success" @click="setPlan(row, 'done')">完成</el-button>
          <el-button v-if="row.status === 'done'" size="small" @click="planEffect(row)">效果</el-button>
          <el-button size="small" type="danger" plain @click="setPlan(row, 'abandoned')">放弃</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="planVisible" title="新建 / 转改版计划" width="560px">
      <el-form label-position="top">
        <el-form-item label="课程码"><el-input v-model="planForm.course_code" /></el-form-item>
        <el-form-item label="问题"><el-input v-model="planForm.problem" /></el-form-item>
        <el-form-item label="依据（必填）"><el-input v-model="planForm.evidence" /></el-form-item>
        <el-form-item label="措施"><el-input v-model="planForm.measure" /></el-form-item>
        <el-form-item label="目标指标（必填）"><el-input v-model="planForm.target_metric" /></el-form-item>
        <el-form-item label="优先级">
          <el-select v-model="planForm.priority"><el-option label="P0" value="P0" /><el-option label="P1" value="P1" /><el-option label="P2" value="P2" /></el-select>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="planVisible = false">取消</el-button><el-button type="primary" @click="submitPlan">提交</el-button></template>
    </el-dialog>
  </el-card>
</template>