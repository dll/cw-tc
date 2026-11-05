<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, ChatDotRound, EditPen, Money, Reading, School, Setting, TrendCharts, User } from '@element-plus/icons-vue'
import { getName, getProfile, getRole, logout, pendingRegistrationCount, type StaffProfile } from '@/api/client'

// 左侧菜单按业务域分组（级联菜单）；进入页面时自动展开当前分组、收起其它分组。
type MenuItem = { path: string; label: string; badge?: 'pending' }
type MenuGroup = { key: string; label: string; icon: unknown; items: MenuItem[] }

const route = useRoute()
const router = useRouter()
const title = computed(() => (route.path.startsWith('/admin/teacher') ? '教师工作台' : '管理后台'))
const isLogin = computed(() => route.path === '/admin/login' || route.path === '/admin/change-password')
const pendingCount = ref(0)
const profile = ref<StaffProfile | null>(null)
const displayName = computed(() => profile.value?.name || getName() || '未登录')
const roleLabel = computed(() => {
  switch (profile.value?.role || getRole()) {
    case 'teacher':
      return '教师'
    case 'admin_super':
      return '超级管理员'
    case 'admin_ops':
      return '管理员'
    default:
      return ''
  }
})

async function loadProfile() {
  if (isLogin.value) return
  try {
    profile.value = await getProfile()
  } catch {
    profile.value = null
  }
}

async function onUserCommand(command: string) {
  if (command === 'profile') {
    void router.push('/admin/profile')
  } else if (command === 'password') {
    void router.push('/admin/change-password')
  } else if (command === 'logout') {
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
}

let pendingTimer: ReturnType<typeof setInterval> | undefined
async function refreshPending() {
  if (isLogin.value) return
  try { pendingCount.value = await pendingRegistrationCount() } catch { pendingCount.value = 0 }
}
onMounted(() => {
  void loadProfile()
  void refreshPending()
  pendingTimer = setInterval(() => { void refreshPending() }, 30000)
})
onUnmounted(() => { if (pendingTimer) clearInterval(pendingTimer) })

const adminGroups: MenuGroup[] = [
  {
    key: 'content',
    label: '内容与课程',
    icon: Reading,
    items: [
      { path: '/admin/courses', label: '课程管理' },
      { path: '/admin/content-requests', label: '下架申请' },
      { path: '/admin/majors', label: '专业分类' },
      { path: '/admin/kb', label: '知识库重建' },
      { path: '/admin/ai-drafts', label: 'AI 内容二审' },
      { path: '/admin/ai-insights', label: 'AI 运营报告' },
      { path: '/admin/public-site', label: '对外展示' },
      { path: '/admin/promo', label: '优惠码' },
      { path: '/admin/discussions', label: '讨论治理' },
    ],
  },
  {
    key: 'operation',
    label: '运营与数据',
    icon: TrendCharts,
    items: [
      { path: '/admin/operations', label: '运营位' },
      { path: '/admin/insights', label: '运营统计' },
      { path: '/admin/comments', label: '评价审核' },
      { path: '/admin/suggestions', label: '学员建议' },
      { path: '/admin/broadcast', label: '广播通知' },
    ],
  },
  {
    key: 'users',
    label: '用户与权限',
    icon: User,
    items: [
      { path: '/admin/users', label: '用户管理', badge: 'pending' },
      { path: '/admin/appeals', label: '账号申诉' },
      { path: '/admin/deletions', label: '注销审核' },
      { path: '/admin/entitlements', label: '权益台账' },
      { path: '/admin/staff', label: '内部人员' },
      { path: '/admin/admins', label: '管理员账号' },
      { path: '/admin/audit', label: '审计日志' },
    ],
  },
  {
    key: 'finance',
    label: '交易与证书',
    icon: Money,
    items: [
      { path: '/admin/orders', label: '订单管理' },
      { path: '/admin/refunds', label: '退款处理' },
      { path: '/admin/reconcile', label: '对账与渠道' },
      { path: '/admin/certs', label: '证书台账' },
    ],
  },
  {
    key: 'security',
    label: '风控与系统',
    icon: Setting,
    items: [
      { path: '/admin/risk', label: '风控与黑名单' },
      { path: '/admin/exam-anomalies', label: '测评异常复核' },
      { path: '/admin/settings', label: '系统配置' },
    ],
  },
]

const teacherGroups: MenuGroup[] = [
  {
    key: 'teaching',
    label: '我的教学',
    icon: School,
    items: [
      { path: '/admin/teacher/courses', label: '我的课程' },
      { path: '/admin/teacher/courses/new', label: '新建课程' },
    ],
  },
  {
    key: 'review',
    label: '评审与阅卷',
    icon: EditPen,
    items: [
      { path: '/admin/teacher/review', label: '阅卷台' },
      { path: '/admin/teacher/ai-drafts', label: 'AI 内容草稿' },
      { path: '/admin/teacher/insights', label: 'AI 周报与改版' },
    ],
  },
  {
    key: 'qa',
    label: '答疑与讨论',
    icon: ChatDotRound,
    items: [
      { path: '/admin/teacher/discussions', label: '讨论与问答' },
    ],
  },
]

const menus = computed<MenuGroup[]>(() => {
  const role = getRole()
  if (role === 'teacher') return teacherGroups
  if (role === 'admin_ops' || role === 'admin_super') return adminGroups
  // 旧会话尚未写入角色时保持兼容；重新登录后按角色收敛。
  return [...adminGroups, ...teacherGroups]
})

function groupKeyForPath(path: string): string {
  for (const group of menus.value) {
    if (group.items.some((item) => item.path === path)) return group.key
  }
  return ''
}

const menuRef = ref<{ open: (index: string) => void; close: (index: string) => void } | null>(null)
const initialOpenGroups = computed(() => {
  const key = groupKeyForPath(route.path)
  return key ? [key] : []
})

watch(
  () => route.path,
  async (next, prev) => {
    const nextKey = groupKeyForPath(next)
    const prevKey = prev ? groupKeyForPath(prev) : ''
    await nextTick()
    if (prevKey && prevKey !== nextKey) menuRef.value?.close(prevKey)
    if (nextKey) menuRef.value?.open(nextKey)
  },
)

watch(isLogin, (login) => {
  if (!login) void loadProfile()
})
</script>

<template>
  <router-view v-if="isLogin" />
  <el-container v-else class="layout">
    <el-aside width="232px" class="aside">
      <div class="brand">第三课堂</div>
      <div class="brand-sub">{{ title }}</div>
      <el-menu
        ref="menuRef"
        :default-active="route.path"
        :default-openeds="initialOpenGroups"
        unique-opened
        router
        class="menu"
      >
        <el-sub-menu v-for="group in menus" :key="group.key" :index="group.key">
          <template #title>
            <el-icon><component :is="group.icon" /></el-icon>
            <span>{{ group.label }}</span>
          </template>
          <el-menu-item v-for="item in group.items" :key="item.path" :index="item.path">
            <span>{{ item.label }}</span>
            <el-badge
              v-if="item.badge === 'pending' && pendingCount > 0"
              :value="pendingCount"
              class="menu-badge"
            />
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>
    <el-container direction="vertical">
      <el-header class="topbar">
        <div class="topbar-title">{{ title }}</div>
        <el-dropdown trigger="click" @command="onUserCommand">
          <span class="user-entry">
            <span class="user-name">{{ displayName }}</span>
            <el-tag v-if="roleLabel" size="small" effect="plain">{{ roleLabel }}</el-tag>
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="profile">个人中心</el-dropdown-item>
              <el-dropdown-item command="password">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout { height: 100vh; }
.aside { border-right: 1px solid var(--el-border-color-light); display: flex; flex-direction: column; }
.brand { padding: 16px 16px 0; font-size: 16px; font-weight: 600; }
.brand-sub { padding: 4px 16px 16px; font-size: 12px; color: var(--el-text-color-secondary); }
.menu { border-right: none; flex: 1; overflow-y: auto; }
.menu-badge { margin-left: auto; }
:deep(.el-menu-item) { display: flex; align-items: center; }
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
}
.topbar-title { font-size: 14px; font-weight: 600; }
.user-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  outline: none;
}
.user-name { font-size: 14px; font-weight: 500; }
</style>
