<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getExamAnomalies, reviewExamAnomaly, type ExamAnomaly } from '@/api/client'

const sessionId = ref('')
const loading = ref(false)
const data = ref<Awaited<ReturnType<typeof getExamAnomalies>> | null>(null)

const kindLabel: Record<string, string> = {
  blur: '切屏 / 离开',
  too_fast: '单题秒答',
  same_option_streak: '连击同选项',
  device_change: '设备突变',
  ip_change: 'IP 突变',
  multi_session: '多会话',
  no_heartbeat: '无心跳',
}
const verdictLabel: Record<string, string> = {
  pending: '待复核',
  kept: '维持成绩',
  invalid: '判无效',
  manual_check: '转人工核验',
}

async function load() {
  const id = Number(sessionId.value)
  if (!id) return
  loading.value = true
  try {
    data.value = await getExamAnomalies(id)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '异常记录读取失败')
    data.value = null
  } finally {
    loading.value = false
  }
}

async function review(row: ExamAnomaly, verdict: 'kept' | 'invalid' | 'manual_check') {
  if (!data.value) return
  try {
    const { value } = await ElMessageBox.prompt('人工复核必须填写理由（写入审计）；异常只是信号，不凭单一信号自动判负。', '复核结论', {
      confirmButtonText: '提交结论',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '理由必填'),
    })
    await reviewExamAnomaly(data.value.exam_session_id, verdict, value.trim(), row.id)
    ElMessage.success('结论已记录')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function reviewAll(verdict: 'kept' | 'invalid' | 'manual_check') {
  if (!data.value) return
  try {
    const { value } = await ElMessageBox.prompt('将本次测评所有待复核异常统一处理；理由必填并写审计。', '批量复核', {
      confirmButtonText: '提交结论',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '理由必填'),
    })
    await reviewExamAnomaly(data.value.exam_session_id, verdict, value.trim())
    ElMessage.success('批量结论已记录')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <el-card>
    <template #header>测评异常复核（人工判定 · 不凭单一信号判负）</template>
    <div style="display: flex; gap: 12px; align-items: center; margin-bottom: 12px">
      <el-input v-model="sessionId" placeholder="测评会话 ID" style="width: 200px" @keyup.enter="load" />
      <el-button type="primary" @click="load">查询</el-button>
    </div>
    <template v-if="data">
      <el-descriptions :column="3" border size="small" style="margin-bottom: 12px">
        <el-descriptions-item label="学员">{{ data.student_name }}（{{ data.student_no }}）</el-descriptions-item>
        <el-descriptions-item label="课程">{{ data.course_code }} · {{ data.course_name }}</el-descriptions-item>
        <el-descriptions-item label="等级 / 成绩">{{ data.level }} / {{ data.total_score ?? '-' }}</el-descriptions-item>
      </el-descriptions>
      <el-alert type="info" :closable="false" :title="data.note" style="margin-bottom: 12px" />
      <el-table v-loading="loading" :data="data.items" size="small">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="信号" width="140">
          <template #default="{ row }">{{ kindLabel[row.kind] ?? row.kind }}</template>
        </el-table-column>
        <el-table-column prop="detail" label="原始信号" min-width="180" />
        <el-table-column prop="at" label="发生时间" width="180" />
        <el-table-column label="结论" width="120">
          <template #default="{ row }">{{ verdictLabel[row.verdict] ?? row.verdict }}</template>
        </el-table-column>
        <el-table-column prop="verdict_reason" label="复核理由" min-width="180" />
        <el-table-column label="操作" width="260">
          <template #default="{ row }">
            <template v-if="row.verdict === 'pending'">
              <el-button size="small" type="success" @click="review(row, 'kept')">维持成绩</el-button>
              <el-button size="small" type="danger" plain @click="review(row, 'invalid')">判无效</el-button>
              <el-button size="small" @click="review(row, 'manual_check')">转人工</el-button>
            </template>
            <span v-else>-</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="data.items.some((item) => item.verdict === 'pending')" style="margin-top: 12px">
        <el-button @click="reviewAll('kept')">全部维持成绩</el-button>
        <el-button type="danger" plain @click="reviewAll('invalid')">全部判无效</el-button>
        <el-button @click="reviewAll('manual_check')">全部转人工</el-button>
      </div>
    </template>
  </el-card>
</template>
