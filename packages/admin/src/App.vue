<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

// M0 骨架：左侧菜单 + 路由出口。菜单项与 router.ts 一一对应，
// M1 起按登录角色（管理员 / 教师）过滤可见项。
const route = useRoute()
const title = computed(() => (route.path.startsWith('/teacher') ? '教师工作台' : '管理后台'))

const menus = [
  { group: '管理端', items: [
    { path: '/admin/courses', label: '课程管理' },
    { path: '/admin/majors', label: '专业分类' },
    { path: '/admin/orders', label: '订单与退款' },
    { path: '/admin/certs', label: '证书台账' },
    { path: '/admin/kb', label: '知识库重建' },
    { path: '/admin/broadcast', label: '广播通知' },
    { path: '/admin/users', label: '用户管理' },
    { path: '/admin/audit', label: '审计日志' },
  ] },
  { group: '教师工作台', items: [
    { path: '/teacher/courses', label: '我的课程' },
    { path: '/teacher/courses/new', label: '新建课程' },
    { path: '/teacher/review', label: '阅卷台' },
  ] },
]
</script>

<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="brand">第三课堂</div>
      <div class="brand-sub">{{ title }}</div>
      <el-menu :default-active="route.path" router>
        <template v-for="m in menus" :key="m.group">
          <el-menu-item-group :title="m.group">
            <el-menu-item v-for="it in m.items" :key="it.path" :index="it.path">
              {{ it.label }}
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
