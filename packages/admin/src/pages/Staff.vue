<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createStaff } from '@/api/client'

const form = ref({ account: '', name: '', role: 'teacher', password: '' })
const loading = ref(false)

async function create() {
  if (!form.value.account || !form.value.name || !form.value.password) {
    ElMessage.warning('请填写账号、姓名与临时密码')
    return
  }
  loading.value = true
  try {
    await createStaff({ ...form.value })
    ElMessage.success('已开通（首次登录强制改密）')
    form.value = { account: '', name: '', role: 'teacher', password: '' }
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '开通失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-card>
    <template #header>开通教师 / 管理员（仅超管可建超管）</template>
    <el-form label-position="top" style="max-width: 560px">
      <el-form-item label="账号">
        <el-input v-model="form.account" />
      </el-form-item>
      <el-form-item label="姓名">
        <el-input v-model="form.name" />
      </el-form-item>
      <el-form-item label="角色">
        <el-select v-model="form.role">
          <el-option label="教师" value="teacher" />
          <el-option label="运营管理员" value="admin_ops" />
          <el-option label="超级管理员" value="admin_super" />
        </el-select>
      </el-form-item>
      <el-form-item label="临时密码（首次登录强制改密）">
        <el-input v-model="form.password" type="password" show-password />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="loading" @click="create">开通</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>
