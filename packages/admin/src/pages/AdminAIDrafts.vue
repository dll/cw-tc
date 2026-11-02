<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { exportAIDraft, getAIDraftEffect, listAdminAIDrafts, publishAdminAIDraft, reviewAdminAIDraft, type AIDraftItem } from '@/api/client'

const rows = ref<AIDraftItem[]>([])
const loading = ref(false)
const status = ref('pending_admin')

async function load() {
  loading.value = true
  try {
    rows.value = await listAdminAIDrafts(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '二审队列加载失败')
  } finally {
    loading.value = false
  }
}

async function effect(row: AIDraftItem) {
  try {
    const data = await getAIDraftEffect(row.id, 'admin')
    ElMessageBox.alert(JSON.stringify(data, null, 2), 'AI 增补效果对比', { confirmButtonText: '关闭' })
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '效果对比失败')
  }
}

async function downloadExport(row: AIDraftItem) {
  try {
    const data = await exportAIDraft(row.id, 'admin')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `ai-draft-${row.id}-aigc.json`
    link.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '导出失败')
  }
}

async function review(row: AIDraftItem, verdict: 'approved' | 'rejected') {
  try {
    let issues: string[] = []
    if (verdict === 'rejected') {
      const { value } = await ElMessageBox.prompt('驳回必须填写问题清单（每行一条）', '二审驳回', {
        inputType: 'textarea', inputValidator: (v: string) => (v && v.trim() ? true : '问题清单必填'),
      })
      issues = value.split('\n').map((v) => v.trim()).filter(Boolean)
    } else {
      await ElMessageBox.confirm('确认二审通过？通过后仍需单独执行上架。', '二审通过', { type: 'warning' })
    }
    await reviewAdminAIDraft(row.id, verdict, issues)
    ElMessage.success('二审已提交')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function publish(row: AIDraftItem) {
  try {
    await ElMessageBox.confirm('将一次性插入全部讲次（章级不许半章），已发证书不受影响。确认上架？', 'AI 草稿上架', { type: 'warning' })
    await publishAdminAIDraft(row.id)
    ElMessage.success('已上架')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>AI 内容二审与上架（双审缺一不可）</template>
    <el-select v-model="status" clearable style="width: 200px; margin-bottom: 12px" @change="load">
      <el-option label="待二审" value="pending_admin" />
      <el-option label="已二审" value="admin_approved" />
      <el-option label="已驳回" value="admin_rejected" />
      <el-option label="全部" value="" />
    </el-select>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column type="expand">
        <template #default="{ row }"><pre style="white-space: pre-wrap; max-height: 360px; overflow: auto">{{ row.content_md }}</pre></template>
      </el-table-column>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="course_code" label="课程码" width="130" />
      <el-table-column prop="draft_scope" label="粒度" width="100" />
      <el-table-column prop="lecture_count" label="讲数" width="80" />
      <el-table-column prop="status" label="状态" width="150" />
      <el-table-column prop="model_version" label="模型版本" width="150" />
      <el-table-column prop="filing_no" label="备案号" width="140" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <template v-if="row.status === 'pending_admin'">
            <el-button size="small" type="success" @click="review(row, 'approved')">通过</el-button>
            <el-button size="small" type="danger" plain @click="review(row, 'rejected')">驳回</el-button>
          </template>
          <el-button v-if="row.status === 'published'" size="small" @click="effect(row)">效果</el-button>
          <el-button v-if="row.status === 'published'" size="small" @click="downloadExport(row)">导出</el-button>
          <el-button v-if="row.status === 'published'" size="small" @click="effect(row)">效果</el-button>
          <el-button v-if="row.status === 'published'" size="small" @click="downloadExport(row)">导出</el-button>
          <el-button v-if="row.status === 'admin_approved'" size="small" type="primary" @click="publish(row)">上架</el-button>
          <span v-if="row.status !== 'pending_admin' && row.status !== 'admin_approved'">-</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>