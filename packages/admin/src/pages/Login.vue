<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login } from '@/api/client'

const router = useRouter()
const account = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!account.value || !password.value) {
    ElMessage.warning('请输入账号与密码')
    return
  }
  loading.value = true
  try {
    await login(account.value, password.value)
    router.replace('/admin/courses')
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '账号或密码错误')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <el-card class="login-card">
      <div class="brand">第三课堂 · 管理后台</div>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="account" placeholder="管理员账号" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="password" type="password" show-password placeholder="密码" @keyup.enter="submit" />
        </el-form-item>
        <el-button type="primary" :loading="loading" style="width: 100%" @click="submit">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped>
.login-wrap {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-fill-color-light);
}
.login-card { width: 360px; }
.brand { font-size: 18px; font-weight: 600; margin-bottom: 16px; }
</style>
