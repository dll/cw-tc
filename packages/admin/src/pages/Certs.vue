<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listCerts, revokeCertificate, type CertItem } from '@/api/client'

const rows = ref<CertItem[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 20
const filters = ref({ status: '', course_code: '', q: '' })

async function load() {
  loading.value = true
  try {
    const result = await listCerts({
      status: filters.value.status || undefined,
      course_code: filters.value.course_code || undefined,
      q: filters.value.q || undefined,
      limit: pageSize,
      offset: (page.value - 1) * pageSize,
    })
    rows.value = result.items
    total.value = result.total
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '证书台账读取失败')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
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
    <template #header>证书台账（手机号脱敏 · 撤销须填原因并写审计）</template>
    <div style="display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 12px">
      <el-input v-model="filters.q" placeholder="证书编号前缀" clearable style="width: 200px" @keyup.enter="search" />
      <el-input v-model="filters.course_code" placeholder="课程码" clearable style="width: 160px" @keyup.enter="search" />
      <el-select v-model="filters.status" placeholder="状态" clearable style="width: 140px" @change="search">
        <el-option label="全部状态" value="" />
        <el-option label="有效" value="valid" />
        <el-option label="已撤销" value="revoked" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="cert_no" label="证书编号" width="180" />
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="student_no" label="学号" width="130" />
      <el-table-column prop="student_name" label="姓名" width="110" />
      <el-table-column prop="course" label="课程码" width="120" />
      <el-table-column prop="level" label="等级" width="80" />
      <el-table-column prop="score" label="分数" width="80" />
      <el-table-column prop="band" label="评定" width="90" />
      <el-table-column prop="status" label="状态" width="90" />
      <el-table-column prop="issued_at" label="签发时间" width="180" />
      <el-table-column prop="expires_at" label="有效期至" width="180" />
      <el-table-column prop="revoked_reason" label="撤销原因" width="130">
        <template #default="{ row }">{{ row.revoked_reason ?? '-' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button v-if="row.status === 'valid'" size="small" type="danger" plain @click="revoke(row)">撤销证书</el-button>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="page"
      :page-size="pageSize"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 16px"
      @current-change="load"
    />
  </el-card>
</template>
