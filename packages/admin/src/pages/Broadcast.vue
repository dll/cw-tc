<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createBroadcast, confirmBroadcast } from '@/api/client'

const form = ref({ target: 'all', target_value: '', title: '', body: '', idempotency_key: '' })
const loading = ref(false)
const lastResult = ref('')

async function create() {
  if (!form.value.title || !form.value.body) {
    ElMessage.warning('请填写标题与正文')
    return
  }
  loading.value = true
  try {
    const { broadcast_id } = await createBroadcast({
      target: form.value.target,
      target_value: form.value.target_value || undefined,
      title: form.value.title,
      body: form.value.body,
      idempotency_key: form.value.idempotency_key || `bc-${Date.now()}`,
    })
    // 创建草稿后二次确认发送
    const { recipients } = await confirmBroadcast(broadcast_id)
    lastResult.value = `已发送，触达 ${recipients} 人`
    ElMessage.success(lastResult.value)
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '发送失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-card>
    <template #header>广播通知（创建即二次确认发送）</template>
    <el-form label-position="top" style="max-width: 560px">
      <el-form-item label="目标">
        <el-select v-model="form.target">
          <el-option label="全体学员" value="all" />
          <el-option label="按课程" value="course" />
          <el-option label="按专业" value="major" />
        </el-select>
      </el-form-item>
      <el-form-item v-if="form.target !== 'all'" label="目标值（课程码 / 专业 ID）">
        <el-input v-model="form.target_value" />
      </el-form-item>
      <el-form-item label="标题">
        <el-input v-model="form.title" />
      </el-form-item>
      <el-form-item label="正文">
        <el-input v-model="form.body" type="textarea" :rows="3" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="loading" @click="create">创建并发送</el-button>
        <span v-if="lastResult" style="margin-left: 12px">{{ lastResult }}</span>
      </el-form-item>
    </el-form>
  </el-card>
</template>
