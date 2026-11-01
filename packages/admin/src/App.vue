<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChatDotRound, EditPen, Money, Reading, School, Setting, TrendCharts, User } from '@element-plus/icons-vue'
import { getRole, pendingRegistrationCount } from '@/api/client'

// 左侧菜单按业务域分组（级联菜单）；进入页面时自动展开当前分组、收起其它分组。
type MenuItem = { path: string; label: string; badge?: 'pending' }
type MenuGroup = { key: string; label: string; icon: unknown; items: MenuItem[] }

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
      { path: '/teacher/courses', label: '我的课程' },
      { path: '/teacher/courses/new', label: '新建课程' },
    ],
  },
  {
    key: 'review',
    label: '评审与阅卷',
    icon: EditPen,
    items: [
      { path: '/teacher/review', label: '阅卷台' },
    ],
  },
  {
    key: 'qa',
    label: '答疑与讨论',
    icon: ChatDotRound,
    items: [
      { path: '/teacher/discussions', label: '讨论与问答' },
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
    <el-main>
      <router-view />
    </el-main>
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
</style>
