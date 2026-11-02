<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createAdmin, disableAdmin, listAdmins, type AdminAccountItem } from '@/api/client'

const rows = ref<AdminAccountItem[]>([])
const loading = ref(false)
const visible = ref(false)
const form = ref({ account: '', name: '', role: 'admin_ops', password: '' })

const roleLabel: Record<string, string> = { admin_super: '超级管理员', admin_ops: '运营管理员' }

async function load() {
  loading.value = true
  try {
    rows.value = await listAdmins()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '管理员列表读取失败')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = { account: '', name: '', role: 'admin_ops', password: '' }
  visible.value = true
}

async function submit() {
  if (!form.value.account || !form.value.name || !form.value.password) return
  try {
    await createAdmin({ ...form.value })
    visible.value = false
    ElMessage.success('管理员已创建，首次登录强制改密')
    await load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '创建失败')
  }
}

async function disable(row: AdminAccountItem) {
  try {
    await ElMessageBox.confirm(`确认停用 ${row.name}（${roleLabel[row.role] ?? row.role}）？禁止停用最后一个超管。`, '高危操作确认', { type: 'warning' })
    await disableAdmin(row.id)
    ElMessage.success('已停用并写审计')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>管理员账号（仅超级管理员可创建 / 停用）</template>
    <el-button type="primary" @click="openCreate" style="margin-bottom: 12px">新建管理员</el-button>
    <el-table v-loading="loading" :data="rows" stripe size="small">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="account" label="账号" width="160" />
      <el-table-column prop="name" label="姓名" width="140" />
      <el-table-column label="角色" width="140">
        <template #default="{ row }">{{ roleLabel[row.role] ?? row.role }}</template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="created_at" label="创建时间" width="180" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button v-if="row.status === 'active'" size="small" type="danger" plain @click="disable(row)">停用</el-button>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="visible" title="新建管理员" width="480px">
      <el-form label-position="top">
        <el-form-item label="账号"><el-input v-model="form.account" /></el-form-item>
        <el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.role">
            <el-option label="运营管理员" value="admin_ops" />
            <el-option label="超级管理员" value="admin_super" />
          </el-select>
        </el-form-item>
        <el-form-item label="临时密码（首次登录强制改密）">
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="submit">创建</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>
