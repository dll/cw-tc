<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTeacherCourses, createTeacherCourse, updateTeacherCourse, submitCourse, listMajors, type AdminCourse, type MajorItem } from '@/api/client'

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
  majors.value = await listMajors().catch(() => [])
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
</script>

<template>
  <el-card>
    <template #header>
      我的课程
      <el-button type="primary" size="small" style="float: right" @click="dialogVisible = true">新建课程</el-button>
    </template>
    <el-table v-loading="loading" :data="rows" stripe>
      <el-table-column prop="course_code" label="课程码" width="200" />
      <el-table-column prop="name" label="课程名" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">{{ statusLabel[row.status] ?? row.status }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button v-if="row.status === 'proposed' || row.status === 'rejected'" size="small" @click="openEdit(row)">
            编辑
          </el-button>
          <el-button v-if="row.status === 'proposed' || row.status === 'rejected'" size="small" type="primary" @click="onSubmit(row)">
            提交送审
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
  </el-card>
</template>
