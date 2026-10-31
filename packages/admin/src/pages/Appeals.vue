<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listAppeals, handleAppeal, type AppealItem } from '@/api/client'

const rows = ref<AppealItem[]>([])
const loading = ref(false)
const status = ref('pending')

const kindLabel: Record<string, string> = {
  ban: '封禁申诉',
  identity: '实名申诉',
  score: '成绩申诉',
  refund: '退款申诉',
}

async function load() {
  loading.value = true
  try {
    rows.value = await listAppeals(status.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '申诉加载失败')
  } finally {
    loading.value = false
  }
}

async function handle(row: AppealItem, verdict: 'accepted' | 'rejected') {
  try {
    const { value } = await ElMessageBox.prompt(
      verdict === 'accepted'
        ? '通过后结果会通知学员；封禁申诉通过将自动解封（不恢复已撤销证书 / 已退款订单）。请填写结论。'
        : '驳回必须填写理由，结果会通知学员。',
      verdict === 'accepted' ? '受理申诉' : '驳回申诉',
      {
        confirmButtonText: '确认提交',
        cancelButtonText: '取消',
        inputType: 'textarea',
        inputValidator: (v: string) => (v && v.trim() ? true : '结论必填'),
      },
    )
    await ElMessageBox.confirm('确认提交处理结论？该操作写入审计并通知学员。', '二次确认', { type: 'warning' })
    await handleAppeal(row.id, verdict, value.trim())
    ElMessage.success('已处理并通知学员')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>账号申诉（手机号脱敏 · 结论必填 · 全程审计）</template>
    <el-radio-group v-model="status" @change="load" style="margin-bottom: 12px">
      <el-radio-button label="pending">待处理</el-radio-button>
      <el-radio-button label="accepted">已受理</el-radio-button>
      <el-radio-button label="rejected">已驳回</el-radio-button>
      <el-radio-button label="">全部</el-radio-button>
    </el-radio-group>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }">{{ kindLabel[row.kind] ?? row.kind }}</template>
      </el-table-column>
      <el-table-column prop="name" label="姓名" width="110">
        <template #default="{ row }">{{ row.name ?? '-' }}</template>
      </el-table-column>
      <el-table-column prop="phone_masked" label="手机号" width="130" />
      <el-table-column prop="content_text" label="申诉内容" min-width="240" />
      <el-table-column prop="created_at" label="提交时间" width="180" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="result_note" label="结论" min-width="180">
        <template #default="{ row }">{{ row.result_note ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <template v-if="row.status === 'pending'">
            <el-button size="small" type="success" @click="handle(row, 'accepted')">受理</el-button>
            <el-button size="small" type="danger" plain @click="handle(row, 'rejected')">驳回</el-button>
          </template>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
