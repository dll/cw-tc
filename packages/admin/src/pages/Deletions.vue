<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { handleDeletionRequest, listDeletionRequests, type DeletionRequestItem } from '@/api/client'

const rows = ref<DeletionRequestItem[]>([])
const loading = ref(false)
const status = ref('pending')

async function load() {
  loading.value = true
  try {
    rows.value = await listDeletionRequests(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '注销待办加载失败')
  } finally {
    loading.value = false
  }
}

async function handle(row: DeletionRequestItem, approve: boolean) {
  try {
    const { value } = await ElMessageBox.prompt(
      approve
        ? '通过后将立即匿名化：姓名脱敏、手机号置空、身份证密文与哈希删除；证书与订单台账保留。请填写结论。'
        : '驳回将恢复账号；请填写理由并通知学员。',
      approve ? '通过注销' : '驳回注销',
      {
        confirmButtonText: '确认提交',
        cancelButtonText: '取消',
        inputType: 'textarea',
        inputValidator: (v: string) => (v && v.trim() ? true : '结论必填'),
      },
    )
    await ElMessageBox.confirm('该操作将写入审计；确认继续？', '二次确认', { type: 'warning' })
    await handleDeletionRequest(row.id, approve, value.trim())
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
    <template #header>注销审核（通过才匿名化 · 证书与订单台账保留）</template>
    <el-radio-group v-model="status" @change="load" style="margin-bottom: 12px">
      <el-radio-button label="pending">待处理</el-radio-button>
      <el-radio-button label="approved">已通过</el-radio-button>
      <el-radio-button label="rejected">已驳回</el-radio-button>
      <el-radio-button label="">全部</el-radio-button>
    </el-radio-group>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="student_no" label="学号" width="130" />
      <el-table-column prop="name" label="姓名" width="110" />
      <el-table-column prop="phone_masked" label="手机号" width="130" />
      <el-table-column prop="reason" label="注销原因" min-width="200" />
      <el-table-column prop="created_at" label="申请时间" width="180" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="result_note" label="结论" min-width="180">
        <template #default="{ row }">{{ row.result_note ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <template v-if="row.status === 'pending'">
            <el-button size="small" type="success" @click="handle(row, true)">通过</el-button>
            <el-button size="small" type="danger" plain @click="handle(row, false)">驳回</el-button>
          </template>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
