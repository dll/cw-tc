<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listOperationSlots,
  createOperationSlot,
  updateOperationSlot,
  removeOperationSlot,
  type OperationSlotInput,
  type OperationSlotItem,
} from '@/api/client'

const rows = ref<OperationSlotItem[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const editId = ref<number | null>(null)

const positionLabel: Record<string, string> = {
  home_banner: '首页 Banner',
  home_recommend: '首页推荐',
  plaza_top: '广场置顶',
  topic: '专题',
}
const statusLabel: Record<string, string> = {
  draft: '草稿',
  active: '生效中',
  expired: '已过期',
  removed: '已移除',
}

function emptyForm(): OperationSlotInput {
  const now = new Date()
  const end = new Date(now.getTime() + 7 * 24 * 3600 * 1000)
  return {
    position: 'home_banner',
    course_code: '',
    title: '',
    image_url: '',
    link_url: '',
    starts_at: toLocalInput(now.toISOString()),
    ends_at: toLocalInput(end.toISOString()),
    weight: 0,
    status: 'active',
  }
}
const form = ref<OperationSlotInput>(emptyForm())

function toLocalInput(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
function toISO(local: string): string {
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString()
}

async function load() {
  loading.value = true
  try {
    rows.value = await listOperationSlots()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '运营位加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openCreate() {
  editId.value = null
  form.value = emptyForm()
  dialogVisible.value = true
}
function openEdit(row: OperationSlotItem) {
  editId.value = row.id
  form.value = {
    position: row.position,
    course_code: row.course_code ?? '',
    title: row.title,
    image_url: row.image_url ?? '',
    link_url: row.link_url ?? '',
    starts_at: toLocalInput(row.starts_at),
    ends_at: toLocalInput(row.ends_at),
    weight: row.weight,
    status: row.status === 'removed' || row.status === 'expired' ? 'active' : row.status,
  }
  dialogVisible.value = true
}

async function submit() {
  if (!form.value.title.trim() && !form.value.image_url.trim()) {
    ElMessage.warning('标题与图片至少填写一项')
    return
  }
  if ((form.value.position === 'home_recommend' || form.value.position === 'plaza_top') && !form.value.course_code.trim()) {
    ElMessage.warning('推荐位 / 置顶位必须关联课程')
    return
  }
  const payload: OperationSlotInput = {
    ...form.value,
    starts_at: toISO(form.value.starts_at),
    ends_at: toISO(form.value.ends_at),
  }
  if (!payload.starts_at || !payload.ends_at) {
    ElMessage.warning('请填写起止时间')
    return
  }
  try {
    if (editId.value) {
      await updateOperationSlot(editId.value, payload)
    } else {
      await createOperationSlot(payload)
    }
    ElMessage.success('已保存')
    dialogVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  }
}

async function onRemove(row: OperationSlotItem) {
  try {
    await ElMessageBox.confirm(`确认下线运营位「${row.title || row.course_code}」？记录保留但不再展示。`, '下线运营位', { type: 'warning' })
    await removeOperationSlot(row.id)
    ElMessage.success('已下线')
    load()
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}
</script>

<template>
  <el-card>
    <template #header>
      运营位（Banner / 推荐 / 置顶 / 专题；到期自动下线，课程下架自动移除）
      <el-button type="primary" size="small" style="float: right" @click="openCreate">新建运营位</el-button>
    </template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column label="位置" width="130">
        <template #default="{ row }">{{ positionLabel[row.position] ?? row.position }}</template>
      </el-table-column>
      <el-table-column prop="course_code" label="关联课程" width="130" />
      <el-table-column prop="title" label="标题" min-width="160" />
      <el-table-column prop="starts_at" label="开始" width="180" />
      <el-table-column prop="ends_at" label="结束" width="180" />
      <el-table-column prop="weight" label="权重" width="80" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button v-if="row.status !== 'removed'" size="small" type="danger" plain @click="onRemove(row)">下线</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editId ? '编辑运营位' : '新建运营位'" width="560px">
      <el-form label-position="top">
        <el-form-item label="位置">
          <el-select v-model="form.position" style="width: 100%">
            <el-option v-for="(label, value) in positionLabel" :key="value" :label="label" :value="value" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联课程码（Banner / 专题可留空；推荐 / 置顶必填且课程须已上架）">
          <el-input v-model="form.course_code" />
        </el-form-item>
        <el-form-item label="标题"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="图片 URL"><el-input v-model="form.image_url" /></el-form-item>
        <el-form-item label="跳转链接"><el-input v-model="form.link_url" /></el-form-item>
        <el-form-item label="开始时间">
          <el-input v-model="form.starts_at" type="datetime-local" />
        </el-form-item>
        <el-form-item label="结束时间">
          <el-input v-model="form.ends_at" type="datetime-local" />
        </el-form-item>
        <el-form-item label="权重（越大越靠前）">
          <el-input-number v-model="form.weight" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="草稿" value="draft" />
            <el-option label="生效中" value="active" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>
