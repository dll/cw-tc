<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminCourseStats, adminInsights, type CourseStats } from '@/api/client'

interface FunnelStep {
  key: string
  label: string
  value: number
  rate_from_previous: number | null
}

const rows = ref<CourseStats[]>([])
const loading = ref(false)
const source = ref('')
const funnel = ref<{ course_code: string; data_as_of: string | null; items: FunnelStep[] } | null>(null)
const funnelOpen = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await adminInsights(source.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '运营统计读取失败')
  } finally {
    loading.value = false
  }
}

async function openFunnel(row: CourseStats) {
  try {
    const detail = await adminCourseStats(row.course_code)
    funnel.value = { course_code: row.course_code, data_as_of: detail.data_as_of, items: detail.funnel.items }
    funnelOpen.value = true
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '漏斗读取失败')
  }
}

function percent(rate: number | null): string {
  if (rate === null || Number.isNaN(rate)) return '-'
  return `${(rate * 100).toFixed(1)}%`
}

onMounted(load)
</script>

<template>
  <div>
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="统计为 T+1 跑批；人数一律去重。页面必须显示“数据截至”日期，避免把数字当成实时值。"
      style="margin-bottom: 12px"
    />
    <el-radio-group v-model="source" @change="load" style="margin-bottom: 12px">
      <el-radio-button label="">全部来源</el-radio-button>
      <el-radio-button label="upstream">上游课程</el-radio-button>
      <el-radio-button label="platform">平台课程</el-radio-button>
    </el-radio-group>
    <el-table v-loading="loading" :data="rows" size="small" @row-click="openFunnel">
      <el-table-column prop="course_code" label="课程编码" width="140" />
      <el-table-column prop="name" label="课程" min-width="180" />
      <el-table-column prop="data_as_of" label="数据截至" width="120">
        <template #default="{ row }">{{ row.data_as_of ?? '未跑批' }}</template>
      </el-table-column>
      <el-table-column prop="views_uv" label="浏览" width="80" />
      <el-table-column prop="favorites" label="收藏" width="80" />
      <el-table-column prop="learning" label="在学" width="80" />
      <el-table-column prop="completed" label="学完" width="80" />
      <el-table-column prop="examined" label="已考" width="80" />
      <el-table-column prop="certified" label="获证" width="80" />
      <el-table-column prop="avg_rating" label="均分" width="90">
        <template #default="{ row }">{{ row.avg_rating ?? '-' }}（{{ row.rating_count }} 条）</template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="funnelOpen" :title="funnel ? `转化漏斗 · ${funnel.course_code}` : '转化漏斗'" size="420px">
      <template v-if="funnel">
        <p>数据截至：{{ funnel.data_as_of ?? '未跑批' }}</p>
        <el-table :data="funnel.items" size="small">
          <el-table-column prop="label" label="环节" width="110" />
          <el-table-column prop="value" label="人数" width="90" />
          <el-table-column label="较上一环节" width="120">
            <template #default="{ row }">{{ percent(row.rate_from_previous) }}</template>
          </el-table-column>
        </el-table>
      </template>
    </el-drawer>
  </div>
</template>
