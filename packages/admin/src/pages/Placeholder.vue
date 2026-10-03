<script setup lang="ts">
defineProps<{ title: string; detail?: string }>()
</script>

<template>
  <div>
    <h2 class="page-title">{{ title }}</h2>
    <el-card v-if="detail" shadow="never">
      <p class="page-detail">{{ detail }}</p>
      <el-tag type="info" size="small">待实现</el-tag>
    </el-card>
  </div>
</template>

<style scoped>
.page-title { margin: 0 0 16px; font-size: 18px; font-weight: 600; }
.page-detail { margin: 0 0 12px; color: var(--el-text-color-regular); }
</style>
