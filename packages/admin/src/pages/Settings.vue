<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listSettings, updateSetting, type SystemConfigItem } from '@/api/client'

const rows = ref<SystemConfigItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await listSettings()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '配置读取失败')
  } finally {
    loading.value = false
  }
}

async function save(row: SystemConfigItem) {
  try {
    await updateSetting(row.key, Number(row.value))
    ElMessage.success('已保存（改后无需发版即生效）')
    await load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="系统配置只放业务参数；密钥只走服务器 shared/.env，本页与接口均不回显。"
      style="margin-bottom: 12px"
    />
    <el-table v-loading="loading" :data="rows" size="small">
      <el-table-column prop="key" label="配置键" width="240" />
      <el-table-column prop="description" label="说明" min-width="260" />
      <el-table-column label="当前值" width="180">
        <template #default="{ row }">
          <el-input-number v-model="row.value" :min="row.min_value ?? undefined" :max="row.max_value ?? undefined" :step="1" />
        </template>
      </el-table-column>
      <el-table-column label="边界" width="120">
        <template #default="{ row }">
          {{ row.min_value ?? '-' }} ~ {{ row.max_value ?? '不限' }}
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="最近修改" width="200" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button size="small" type="primary" @click="save(row)">保存</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
