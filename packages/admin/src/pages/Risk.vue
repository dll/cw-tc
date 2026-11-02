<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  addBlacklist,
  listRisk,
  resolveBlacklistAppeal,
  type RiskBlacklistItem,
} from '@/api/client'

const rows = ref<RiskBlacklistItem[]>([])
const loading = ref(false)
const total = ref(0)
const rules = ref({ refund_abuse_limit: 0, refund_abuse_window_days: 0, note: '' })
const filters = ref({ status: '', target_type: '' })
const dialogVisible = ref(false)
const form = ref({ target_type: 'phone', target_value: '', reason: '', expires_at: '' })

const typeLabel: Record<string, string> = { phone: '手机号', device: '设备', payment_account: '支付账户' }
const statusLabel: Record<string, string> = { active: '生效中', expired: '已解除', appealed: '申诉中' }

async function load() {
  loading.value = true
  try {
    const result = await listRisk({
      status: filters.value.status || undefined,
      target_type: filters.value.target_type || undefined,
    })
    rows.value = result.items
    total.value = result.total
    rules.value = result.rules
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '风控数据加载失败')
  } finally {
    loading.value = false
  }
}

function openAdd() {
  const until = new Date(Date.now() + 30 * 24 * 3600 * 1000).toISOString().slice(0, 16)
  form.value = { target_type: 'phone', target_value: '', reason: '', expires_at: until }
  dialogVisible.value = true
}

async function submitAdd() {
  if (!form.value.target_value.trim() || !form.value.reason.trim() || !form.value.expires_at) return
  try {
    await ElMessageBox.confirm('加入黑名单必须有期限且可申诉，确认执行？', '高危操作确认', { type: 'warning' })
    await addBlacklist({
      target_type: form.value.target_type,
      target_value: form.value.target_value.trim(),
      reason: form.value.reason.trim(),
      expires_at: new Date(form.value.expires_at).toISOString(),
      confirm: true,
    })
    dialogVisible.value = false
    ElMessage.success('已加入黑名单并写审计')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function resolve(row: RiskBlacklistItem, accept: boolean) {
  try {
    const { value } = await ElMessageBox.prompt(accept ? '受理申诉将解除黑名单；请填写结论。' : '驳回申诉将恢复黑名单；请填写结论。', '处理申诉', {
      confirmButtonText: '确认提交',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '结论必填'),
    })
    await resolveBlacklistAppeal(row.id, accept, value.trim())
    ElMessage.success('已处理并写审计')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>风控与黑名单（必须有期限 · 可申诉 · 手机号脱敏）</template>
    <el-alert
      :closable="false"
      type="info"
      :title="`退款自助上限：${rules.refund_abuse_limit} 次 / ${rules.refund_abuse_window_days} 天；${rules.note}`"
      style="margin-bottom: 12px"
    />
    <div style="display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 12px">
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 150px" @change="load">
        <el-option label="全部状态" value="" />
        <el-option label="生效中" value="active" />
        <el-option label="申诉中" value="appealed" />
        <el-option label="已解除" value="expired" />
      </el-select>
      <el-select v-model="filters.target_type" placeholder="对象类型" clearable style="width: 160px" @change="load">
        <el-option label="全部对象" value="" />
        <el-option label="手机号" value="phone" />
        <el-option label="设备" value="device" />
        <el-option label="支付账户" value="payment_account" />
      </el-select>
      <el-button type="primary" @click="openAdd">加入黑名单</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="对象类型" width="100">
        <template #default="{ row }">{{ typeLabel[row.target_type] ?? row.target_type }}</template>
      </el-table-column>
      <el-table-column prop="target_value_masked" label="对象（脱敏）" min-width="160" />
      <el-table-column prop="reason" label="原因" min-width="180" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'danger' : row.status === 'appealed' ? 'warning' : 'info'" size="small">
            {{ statusLabel[row.status] ?? row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="expires_at" label="期限" width="180" />
      <el-table-column prop="appeal_reason" label="申诉理由" min-width="180" />
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <template v-if="row.status === 'appealed'">
            <el-button size="small" type="success" @click="resolve(row, true)">受理解除</el-button>
            <el-button size="small" type="danger" plain @click="resolve(row, false)">驳回申诉</el-button>
          </template>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
    <p style="color: var(--el-text-color-secondary); font-size: 12px">共 {{ total }} 条</p>

    <el-dialog v-model="dialogVisible" title="加入黑名单" width="520px">
      <el-form label-position="top">
        <el-form-item label="对象类型">
          <el-select v-model="form.target_type">
            <el-option label="手机号" value="phone" />
            <el-option label="设备" value="device" />
            <el-option label="支付账户" value="payment_account" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象值（手机号 / 设备标识 / 支付账户）">
          <el-input v-model="form.target_value" />
        </el-form-item>
        <el-form-item label="原因（必填）">
          <el-input v-model="form.reason" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="期限（必填，必须有期限）">
          <el-date-picker v-model="form.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="danger" @click="submitAdd">确认加入</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>
