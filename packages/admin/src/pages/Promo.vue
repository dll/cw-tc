<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createPromo, listPromo, setPromoStatus, type PromoItem } from '@/api/client'

const rows = ref<PromoItem[]>([])
const loading = ref(false)
const visible = ref(false)
const form = ref({ code: '', kind: 'amount_off', value: 100, total_limit: 100, per_student_limit: 1, valid_from: '', valid_to: '' })

async function load() {
  loading.value = true
  try {
    rows.value = await listPromo('')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '优惠码加载失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  const now = new Date()
  const end = new Date(now.getTime() + 30 * 24 * 3600 * 1000)
  form.value = { code: '', kind: 'amount_off', value: 100, total_limit: 100, per_student_limit: 1, valid_from: now.toISOString(), valid_to: end.toISOString() }
  visible.value = true
}

async function submit() {
  try {
    await createPromo({ ...form.value })
    visible.value = false
    ElMessage.success('优惠码已创建')
    await load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '创建失败')
  }
}

async function toggle(row: PromoItem, status: 'active' | 'disabled') {
  try {
    await ElMessageBox.confirm(`确认${status === 'active' ? '启用' : '停用'}优惠码 ${row.code}？`, '优惠码状态', { type: 'warning' })
    await setPromoStatus(row.code, status)
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
    <template #header>优惠码（单人限用 + 总量上限 + 有效期）</template>
    <el-button type="primary" @click="openCreate" style="margin-bottom: 12px">新建优惠码</el-button>
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="code" label="优惠码" width="150" />
      <el-table-column prop="kind" label="类型" width="120" />
      <el-table-column prop="value" label="面值 / 折扣" width="120" />
      <el-table-column prop="total_limit" label="总量" width="90" />
      <el-table-column prop="per_student_limit" label="单人" width="90" />
      <el-table-column prop="used" label="已用" width="90" />
      <el-table-column prop="remaining" label="剩余" width="90" />
      <el-table-column prop="valid_from" label="开始" width="180" />
      <el-table-column prop="valid_to" label="结束" width="180" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button v-if="row.status === 'active'" size="small" type="danger" plain @click="toggle(row, 'disabled')">停用</el-button>
          <el-button v-else size="small" type="success" @click="toggle(row, 'active')">启用</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" title="新建优惠码" width="520px">
      <el-form label-position="top">
        <el-form-item label="优惠码"><el-input v-model="form.code" /></el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.kind"><el-option label="固定减免（分）" value="amount_off" /><el-option label="折扣（千分比，850=8.5折）" value="percent_off" /></el-select>
        </el-form-item>
        <el-form-item label="面值"><el-input-number v-model="form.value" :min="1" /></el-form-item>
        <el-form-item label="总量上限"><el-input-number v-model="form.total_limit" :min="1" /></el-form-item>
        <el-form-item label="单人限用"><el-input-number v-model="form.per_student_limit" :min="1" /></el-form-item>
        <el-form-item label="生效时间 ISO"><el-input v-model="form.valid_from" /></el-form-item>
        <el-form-item label="结束时间 ISO"><el-input v-model="form.valid_to" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="visible = false">取消</el-button><el-button type="primary" @click="submit">创建</el-button></template>
    </el-dialog>
  </el-card>
</template>