<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getRole, pendingRegistrationCount } from '@/api/client'

// M0 骨架：左侧菜单 + 路由出口。菜单项与 router.ts 一一对应，
// M1 起按登录角色（管理员 / 教师）过滤可见项。
const route = useRoute()
const title = computed(() => (route.path.startsWith('/teacher') ? '教师工作台' : '管理后台'))
const isLogin = computed(() => route.path === '/login' || route.path === '/change-password')
const pendingCount = ref(0)
let pendingTimer: ReturnType<typeof setInterval> | undefined
async function refreshPending() {
  if (isLogin.value) return
  try { pendingCount.value = await pendingRegistrationCount() } catch { pendingCount.value = 0 }
}
onMounted(() => {
  void refreshPending()
  pendingTimer = setInterval(() => { void refreshPending() }, 30000)
})
onUnmounted(() => { if (pendingTimer) clearInterval(pendingTimer) })

const adminMenus = { group: '管理端', items: [
  { path: '/admin/courses', label: '课程管理' },
  { path: '/admin/content-requests', label: '下架申请' },
  { path: '/admin/operations', label: '运营位' },
  { path: '/admin/insights', label: '运营统计' },
  { path: '/admin/entitlements', label: '权益台账' },
  { path: '/admin/settings', label: '系统配置' },
  { path: '/admin/majors', label: '专业分类' },
  { path: '/admin/orders', label: '订单与退款' },
  { path: '/admin/certs', label: '证书台账' },
  { path: '/admin/comments', label: '评价审核' },
  { path: '/admin/suggestions', label: '学员建议' },
  { path: '/admin/kb', label: '知识库重建' },
  { path: '/admin/broadcast', label: '广播通知' },
  { path: '/admin/users', label: '用户管理' },
  { path: '/admin/staff', label: '内部人员' },
  { path: '/admin/audit', label: '审计日志' },
] }
const teacherMenus = { group: '教师工作台', items: [
  { path: '/teacher/courses', label: '我的课程' },
  { path: '/teacher/courses/new', label: '新建课程' },
  { path: '/teacher/review', label: '阅卷台' },
] }

const menus = computed(() => {
  const role = getRole()
  if (role === 'teacher') return [teacherMenus]
  if (role === 'admin_ops' || role === 'admin_super') return [adminMenus]
  // 旧会话尚未写入角色时保持兼容；重新登录后按角色收敛。
  return [adminMenus, teacherMenus]
})
</script>

<template>
  <router-view v-if="isLogin" />
  <el-container v-else class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">第三课堂</div>
      <div class="brand-sub">{{ title }}</div>
      <el-menu :default-active="route.path" router>
        <template v-for="m in menus" :key="m.group">
          <el-menu-item-group :title="m.group">
            <el-menu-item v-for="it in m.items" :key="it.path" :index="it.path">
              {{ it.label }}
              <el-badge v-if="it.path === '/admin/users' && pendingCount > 0" :value="pendingCount" style="margin-left: 10px" />
            </el-menu-item>
          </el-menu-item-group>
        </template>
      </el-menu>
    </el-aside>
    <el-main>
      <router-view />
    </el-main>
  </el-container>
</template>

<style scoped>
.layout { height: 100vh; }
.aside { border-right: 1px solid var(--el-border-color-light); }
.brand { padding: 16px 16px 0; font-size: 16px; font-weight: 600; }
.brand-sub { padding: 4px 16px 16px; font-size: 12px; color: var(--el-text-color-secondary); }
</style>

