<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getProfile, logout, type StaffProfile } from '@/api/client'

const router = useRouter()
const profile = ref<StaffProfile | null>(null)
const loading = ref(true)

const ROLE_LABELS: Record<string, string> = {
  teacher: '教师',
  admin_ops: '管理员',
  admin_super: '超级管理员',
}

function roleLabel(role: string | undefined): string {
  return role ? ROLE_LABELS[role] ?? role : '-'
}

async function load() {
  loading.value = true
  try {
    profile.value = await getProfile()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '个人信息加载失败')
  } finally {
    loading.value = false
  }
}

async function handleLogout() {
  try {
    await ElMessageBox.confirm('确定退出登录吗？', '退出登录', { type: 'warning' })
  } catch {
    return
  }
  try {
    await logout()
  } catch {
    // 会话可能已失效，本地凭证已在 logout 中清除
  }
  ElMessage.success('已退出登录')
  void router.replace('/admin/login')
}

onMounted(load)
</script>

<template>
  <el-card v-loading="loading" class="profile-card">
    <template #header>
      <div class="card-header">
        <span>个人中心</span>
        <el-button size="small" @click="load">刷新</el-button>
      </div>
    </template>
    <el-descriptions v-if="profile" :column="1" border>
      <el-descriptions-item label="姓名">{{ profile.name }}</el-descriptions-item>
      <el-descriptions-item label="登录账号">{{ profile.account }}</el-descriptions-item>
      <el-descriptions-item label="角色">{{ roleLabel(profile.role) }}</el-descriptions-item>
      <el-descriptions-item label="账号状态">
        <el-tag v-if="!profile.must_change_password" type="success" size="small">正常</el-tag>
        <el-tag v-else type="warning" size="small">待修改初始密码</el-tag>
      </el-descriptions-item>
    </el-descriptions>
    <el-alert
      v-if="profile?.must_change_password"
      class="tip"
      type="warning"
      :closable="false"
      show-icon
      title="当前仍在使用初始密码，请尽快修改"
    />
    <div class="actions">
      <el-button type="primary" @click="router.push('/admin/change-password')">修改密码</el-button>
      <el-button type="danger" plain @click="handleLogout">退出登录</el-button>
    </div>
  </el-card>
</template>

<style scoped>
.profile-card { max-width: 720px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.tip { margin-top: 16px; }
.actions { margin-top: 20px; display: flex; gap: 12px; }
</style>
