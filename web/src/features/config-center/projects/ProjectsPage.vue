<template>
  <section
    class="projects-page"
    aria-labelledby="projects-title"
  >
    <header class="projects-page__header">
      <div>
        <div class="projects-page__title-row">
          <h1 id="projects-title">
            项目
          </h1>
          <EnvironmentTag :environment="environment" />
        </div>
        <p>每个项目独立管理配置和运行时访问权限。</p>
      </div>
      <el-button
        v-if="canWrite"
        type="primary"
        @click="openCreate"
      >
        创建项目
      </el-button>
    </header>

    <el-card shadow="never">
      <el-form
        class="projects-page__filters"
        inline
        size="small"
        @submit.native.prevent="reload"
      >
        <el-form-item label="项目名称">
          <el-input
            v-model.trim="keyword"
            clearable
            placeholder="按名称或描述搜索"
            @clear="reload"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="loading"
            @click="reload"
          >
            搜索
          </el-button>
        </el-form-item>
      </el-form>

      <div
        v-loading="loading"
        class="projects-page__grid"
      >
        <article
          v-for="project in projects"
          :key="String(project.id)"
          class="project-card"
          tabindex="0"
          @click="openProject(project)"
          @keyup.enter="openProject(project)"
        >
          <div class="project-card__heading">
            <div>
              <h2>{{ project.name }}</h2>
              <code>{{ project.key }}</code>
            </div>
            <el-button
              v-if="canWrite"
              aria-label="编辑项目"
              icon="el-icon-edit"
              type="text"
              @click.stop="openEdit(project)"
            />
          </div>
          <p>{{ project.description || '暂无描述' }}</p>
        </article>
      </div>

      <el-empty
        v-if="!loading && projects.length === 0"
        description="当前环境还没有项目"
      />
    </el-card>

    <el-dialog
      :visible.sync="dialog.visible"
      :title="dialog.editing ? '编辑项目' : '创建项目'"
      width="520px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form
        v-if="dialog.visible"
        ref="projectForm"
        :model="dialog.form"
        :rules="rules"
        label-width="88px"
      >
        <el-form-item
          label="项目标识"
          prop="key"
        >
          <el-input
            v-model.trim="dialog.form.key"
            :disabled="dialog.editing"
            maxlength="64"
            placeholder="例如 DemoConfig"
          />
        </el-form-item>
        <el-form-item
          label="项目名称"
          prop="name"
        >
          <el-input
            v-model.trim="dialog.form.name"
            maxlength="64"
            show-word-limit
          />
        </el-form-item>
        <el-form-item
          label="项目描述"
          prop="description"
        >
          <el-input
            v-model="dialog.form.description"
            type="textarea"
            :rows="3"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="dialog.saving"
          @click="save"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import { createProject, updateProject } from '@/api/projects'
import EnvironmentTag from '@/components/EnvironmentTag'

function emptyProject() {
  return { key: '', name: '', description: '' }
}

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

export default {
  name: 'ProjectsPage',

  components: { EnvironmentTag },

  data() {
    return {
      keyword: '',
      loading: false,
      loadSequence: 0,
      projects: [],
      dialog: {
        visible: false,
        editing: false,
        saving: false,
        form: emptyProject(),
      },
      rules: {
        key: [
          { required: true, message: '请输入项目标识', trigger: 'blur' },
          {
            pattern: /^[A-Za-z][A-Za-z0-9]*$/,
            message: '项目标识只能包含字母和数字，且以字母开头',
            trigger: 'blur',
          },
        ],
        name: [
          { required: true, message: '请输入项目名称', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    environment() {
      return this.$store.getters['environment/current']
    },
    canWrite() {
      return this.$store.getters['environment/hasPermission'](
        'project:write',
      )
    },
  },

  watch: {
    environmentId: {
      immediate: true,
      handler(environmentId) {
        if (environmentId) {
          this.reload()
        } else {
          this.projects = []
        }
      },
    },
  },

  methods: {
    async reload() {
      if (!this.environmentId) {
        throw new Error('当前没有可用环境')
      }
      const sequence = ++this.loadSequence
      this.loading = true
      try {
        const reply = await this.$store.dispatch(
          'configCenter/loadProjects',
          {
            environmentId: this.environmentId,
            filter: { keyword: this.keyword },
            force: true,
          },
        )
        if (sequence === this.loadSequence) {
          this.projects = reply.list
        }
      } catch (error) {
        this.$message.error(errorMessage(error, '加载项目失败'))
      } finally {
        if (sequence === this.loadSequence) {
          this.loading = false
        }
      }
    },
    openProject(project) {
      this.$router.push({
        name: 'project-configs',
        params: { projectId: String(project.id) },
      })
    },
    openCreate() {
      this.dialog.editing = false
      this.dialog.form = emptyProject()
      this.dialog.visible = true
    },
    openEdit(project) {
      this.dialog.editing = true
      this.dialog.form = {
        id: project.id,
        key: project.key,
        name: project.name,
        description: project.description || '',
        version: project.version,
      }
      this.dialog.visible = true
    },
    async save() {
      const valid = await new Promise((resolve) => {
        this.$refs.projectForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.dialog.saving = true
      try {
        if (this.dialog.editing) {
          await updateProject(this.environmentId, this.dialog.form)
        } else {
          await createProject(this.environmentId, this.dialog.form)
        }
        this.dialog.visible = false
        this.$message.success(this.dialog.editing ? '项目已更新' : '项目已创建')
        await this.reload()
      } catch (error) {
        this.$message.error(errorMessage(error, '保存项目失败'))
      } finally {
        this.dialog.saving = false
      }
    },
  },
}
</script>

<style scoped>
.projects-page {
  display: grid;
  gap: 20px;
}

.projects-page__header,
.projects-page__title-row,
.project-card__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.projects-page__title-row {
  justify-content: flex-start;
}

.projects-page h1,
.project-card h2 {
  margin: 0;
}

.projects-page__header p,
.project-card p {
  margin: 8px 0 0;
  color: #6b7280;
}

.projects-page__filters {
  margin-bottom: 8px;
}

.projects-page__grid {
  display: grid;
  min-height: 120px;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}

.project-card {
  padding: 20px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.project-card:hover,
.project-card:focus {
  border-color: #409eff;
  box-shadow: 0 8px 24px rgb(15 23 42 / 8%);
  outline: none;
}

.project-card h2 {
  color: #111827;
  font-size: 17px;
}

.project-card code {
  color: #6b7280;
  font-size: 12px;
}
</style>
