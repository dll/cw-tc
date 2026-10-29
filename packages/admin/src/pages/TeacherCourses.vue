<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTeacherCourses, createTeacherCourse, updateTeacherCourse, deleteTeacherCourse, submitCourse, listPublicMajors, createContentRequest, listMyContentRequests, type AdminCourse, type ContentRequestItem, type MajorItem } from '@/api/client'

const rows = ref<AdminCourse[]>([])
const loading = ref(false)
const majors = ref<MajorItem[]>([])

const statusLabel: Record<string, string> = {
  proposed: '草稿', building: '建设中', submitted: '待审', rejected: '已驳回',
  approved: '待上架', published: '已上架', unpublished: '已下架',
}

async function load() {
  loading.value = true
  try {
    rows.value = await listTeacherCourses()
  } finally {
    loading.value = false
  }
}
async function loadMajors() {
  majors.value = await listPublicMajors().catch(() => [])
}
onMounted(() => {
  load()
  loadMajors()
})

const dialogVisible = ref(false)
const form = ref({ course_code: '', name: '', line: 'main', major_id: null as number | null, intro: '' })

async function onCreate() {
  if (!form.value.name.trim() || !form.value.course_code.trim()) {
    ElMessage.warning('请输入课程名与课程码')
    return
  }
  try {
    await createTeacherCourse({
      course_code: form.value.course_code.trim(),
      name: form.value.name.trim(),
      line: form.value.line,
      major_id: form.value.major_id,
      intro: form.value.intro,
    })
    ElMessage.success('草稿已创建')
    dialogVisible.value = false
    form.value = { course_code: '', name: '', line: 'main', major_id: null, intro: '' }
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '创建失败')
  }
}

// 编辑课程（名称/简介/主线/专业分类；课程码创建后不可改）
const editVisible = ref(false)
const editCode = ref('')
const editForm = ref({ name: '', line: 'main', major_id: null as number | null, intro: '' })

function openEdit(row: AdminCourse) {
  editCode.value = row.course_code
  editForm.value = { name: row.name, line: row.line, major_id: row.major_id, intro: row.intro ?? '' }
  editVisible.value = true
}

async function submitEdit() {
  if (!editForm.value.name.trim()) {
    ElMessage.warning('请输入课程名')
    return
  }
  try {
    await updateTeacherCourse(editCode.value, {
      name: editForm.value.name.trim(),
      line: editForm.value.line,
      major_id: editForm.value.major_id,
      intro: editForm.value.intro,
    })
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '保存失败')
  }
}

async function onSubmit(row: AdminCourse) {
  await ElMessageBox.confirm(`提交「${row.name}」送审？提交后由管理员六维审核。`, '提交审核', { type: 'warning' })
  try {
    await submitCourse(row.course_code)
    ElMessage.success('已提交送审')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '提交失败')
  }
}

async function onDelete(row: AdminCourse) {
  await ElMessageBox.confirm(`确认删除「${row.name}」？删除后不可恢复。`, '删除课程', { type: 'warning', confirmButtonText: '删除' })
  try {
    await deleteTeacherCourse(row.course_code)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '删除失败')
  }
}

// —— 下架 / 紧急纠错申请（教师无直接下架权限）——
const requestVisible = ref(false)
const requestTarget = ref<AdminCourse | null>(null)
const requestForm = ref({ kind: 'offline', reason_kind: 'content_error', reason_text: '' })
const myRequestsVisible = ref(false)
const myRequests = ref<ContentRequestItem[]>([])

function openRequest(row: AdminCourse) {
  requestTarget.value = row
  requestForm.value = { kind: 'offline', reason_kind: 'content_error', reason_text: '' }
  requestVisible.value = true
}

async function submitRequest() {
  const target = requestTarget.value
  if (!target) return
  try {
    await ElMessageBox.confirm(
      requestForm.value.kind === 'urgent_fix'
        ? '紧急纠错会通知管理员，并在处理前暂停该课程的新测评/发证；已购学员仍可学习。确认提交？'
        : '下架申请将进入管理员待办，由管理员批准后执行。确认提交？',
      '提交申请',
      { type: 'warning' },
    )
    await createContentRequest(target.course_code, requestForm.value)
    ElMessage.success('申请已提交，等待管理员处理')
    requestVisible.value = false
  } catch (e) {
    if (e instanceof Error && e.message !== 'cancel') ElMessage.error(e.message)
  }
}

async function openMyRequests() {
  try {
    myRequests.value = await listMyContentRequests()
    myRequestsVisible.value = true
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '申请记录加载失败')
  }
}
</script>

<template>
  <el-card>
    <template #header>
      我的课程
      <el-button type="primary" size="small" style="float: right" @click="dialogVisible = true">新建课程</el-button>
      <el-button size="small" style="float: right; margin-right: 8px" @click="openMyRequests">下架申请记录</el-button>
    </template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="course_code" label="课程码" width="200" />
      <el-table-column prop="name" label="课程名" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="操作" width="360">
        <template #default="{ row }">
          <el-button v-if="row.status === 'proposed' || row.status === 'rejected'" size="small" @click="openEdit(row)">
            编辑
          </el-button>
          <el-button v-if="row.status === 'proposed' || row.status === 'rejected'" size="small" type="primary" @click="onSubmit(row)">
            提交审核
          </el-button>
          <el-button v-if="row.status === 'proposed' || row.status === 'rejected'" size="small" type="danger" @click="onDelete(row)">
            删除
          </el-button>
          <el-button v-if="row.status === 'published'" size="small" type="warning" @click="openRequest(row)">
            申请下架
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="新建课程（platform 事实源）" width="480px">
      <el-form label-position="top">
        <el-form-item label="课程码（唯一，创建后不可改）">
          <el-input v-model="form.course_code" placeholder="如 TC-101、DB-ADV" />
        </el-form-item>
        <el-form-item label="课程名">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="主线/辅线">
          <el-select v-model="form.line">
            <el-option label="主线" value="main" />
            <el-option label="辅线" value="aux" />
          </el-select>
        </el-form-item>
        <el-form-item label="专业分类（送审前必填）">
          <el-select v-model="form.major_id" placeholder="请选择专业分类" style="width: 100%">
            <el-option v-for="m in majors" :key="m.id" :label="m.name" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="form.intro" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="onCreate">创建草稿</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editVisible" title="编辑课程" width="480px">
      <el-form label-position="top">
        <el-form-item label="课程码">
          <el-input :model-value="editCode" disabled />
        </el-form-item>
        <el-form-item label="课程名">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="主线/辅线">
          <el-select v-model="editForm.line">
            <el-option label="主线" value="main" />
            <el-option label="辅线" value="aux" />
          </el-select>
        </el-form-item>
        <el-form-item label="专业分类">
          <el-select v-model="editForm.major_id" placeholder="请选择专业分类" style="width: 100%">
            <el-option v-for="m in majors" :key="m.id" :label="m.name" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="简介">
          <el-input v-model="editForm.intro" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="submitEdit">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="requestVisible" title="下架 / 紧急纠错申请" width="480px">
      <el-form label-position="top">
        <el-form-item label="课程">
          <el-input :model-value="requestTarget?.name" disabled />
        </el-form-item>
        <el-form-item label="申请类型">
          <el-select v-model="requestForm.kind" style="width: 100%">
            <el-option label="下架申请" value="offline" />
            <el-option label="紧急纠错（暂停新测评/发证）" value="urgent_fix" />
          </el-select>
        </el-form-item>
        <el-form-item label="原因类别">
          <el-select v-model="requestForm.reason_kind" style="width: 100%">
            <el-option label="内容错误" value="content_error" />
            <el-option label="教材变更" value="material_change" />
            <el-option label="个人原因" value="personal" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="requestForm.reason_text" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="requestVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRequest">提交申请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="myRequestsVisible" title="我的下架申请" width="720px">
      <el-table :data="myRequests" size="small">
        <el-table-column prop="course_name" label="课程" />
        <el-table-column label="类型" width="110">
          <template #default="{ row }">{{ row.kind === 'urgent_fix' ? '紧急纠错' : '下架申请' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">{{ row.status === 'pending' ? '待处理' : row.status === 'approved' ? '已批准' : '已驳回' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="申请时间" width="180" />
      </el-table>
      <el-empty v-if="!myRequests.length" description="暂无申请记录" />
    </el-dialog>
  </el-card>
</template>
