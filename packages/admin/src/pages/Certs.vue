<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCerts, revokeCertificate, type CertItem } from '@/api/client'

const rows = ref<CertItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    rows.value = await listCerts()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '证书台账读取失败')
  } finally {
    loading.value = false
  }
}

async function revoke(row: CertItem) {
  try {
    const { value } = await ElMessageBox.prompt('撤销证书须填写原因（写入审计并通知学员）；已撤销证书不可恢复。', `撤销 ${row.cert_no}`, {
      confirmButtonText: '确认撤销',
      cancelButtonText: '取消',
      inputType: 'textarea',
      inputValidator: (v: string) => (v && v.trim() ? true : '原因必填'),
    })
    await ElMessageBox.confirm('撤销后学员将无法下载证书，验真页显示“已撤销”。确认继续？', '高危操作确认', { type: 'warning' })
    await revokeCertificate(row.cert_no, value.trim())
    ElMessage.success('证书已撤销并通知学员')
    await load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

onMounted(load)
</script>

<template>
  <el-card>
    <template #header>证书台账（手机号脱敏）</template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="cert_no" label="证书编号" width="180" />
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="course" label="课程" width="120" />
      <el-table-column prop="level" label="等级" width="80" />
      <el-table-column prop="score" label="分数" width="80" />
      <el-table-column prop="band" label="评定" width="90" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column prop="issued_at" label="签发时间" width="180" />
      <el-table-column prop="expires_at" label="有效期至" width="180" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button v-if="row.status === 'valid'" size="small" type="danger" plain @click="revoke(row)">撤销证书</el-button>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
