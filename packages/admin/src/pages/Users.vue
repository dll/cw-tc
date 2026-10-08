<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { listUsers, revealUser, type UserItem } from '@/api/client'

const rows = ref<UserItem[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    rows.value = await listUsers()
  } finally {
    loading.value = false
  }
})

async function onReveal(row: UserItem) {
  try {
    const { value } = await ElMessageBox.prompt('看全号必须填写事由（每次都会写审计）', `查看 ${row.name} 手机号`, {
      confirmButtonText: '确认查看',
      cancelButtonText: '取消',
      inputValidator: (v: string) => (v && v.trim() ? true : '事由必填'),
    })
    const { phone } = await revealUser(row.id, value.trim())
    ElMessageBox.alert(phone, '手机号全号', { confirmButtonText: '关闭' })
  } catch {
    /* 用户取消 */
  }
}
</script>

<template>
  <el-card>
    <template #header>用户管理（默认脱敏，看全号需事由并写审计）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="student_no" label="学号" width="120" />
      <el-table-column prop="name" label="姓名" width="120" />
      <el-table-column prop="phone" label="手机号" width="140" />
      <el-table-column prop="status" label="状态" width="100" />
      <el-table-column prop="created_at" label="注册时间" width="180" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button size="small" @click="onReveal(row)">看全号</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
