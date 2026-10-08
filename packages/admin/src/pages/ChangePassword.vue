<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { changePassword } from '@/api/client'

const router = useRouter()
const oldPassword = ref('')
const newPassword = ref('')
const confirm = ref('')
const loading = ref(false)

async function submit() {
  if (!oldPassword.value || !newPassword.value) {
    ElMessage.warning('请输入旧密码与新密码')
    return
  }
  if (newPassword.value !== confirm.value) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  loading.value = true
  try {
    await changePassword(oldPassword.value, newPassword.value)
    ElMessage.success('密码已修改')
    router.replace('/admin/courses')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '修改失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <div class="brand">首次登录，请修改初始密码</div>
      <el-form label-position="top">
        <el-form-item label="旧密码（初始密码）">
          <el-input v-model="oldPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="newPassword" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="confirm" type="password" show-password @keyup.enter="submit" />
        </el-form-item>
        <el-button type="primary" :loading="loading" style="width: 100%" @click="submit">确认修改</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-wrap { height: 100vh; display: flex; align-items: center; justify-content: center; background: var(--el-fill-color-light); }
.login-card { width: 360px; }
.brand { font-size: 18px; font-weight: 600; margin-bottom: 16px; }
</style>
