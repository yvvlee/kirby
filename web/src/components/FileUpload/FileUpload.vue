<template>
  <div
    class="file-upload"
    :class="{
      'file-upload--single-complete': hasFile && !isMultiple,
    }"
  >
    <el-upload
      action="#"
      :accept="acceptType"
      :disabled="disabled"
      :file-list="elementFileList"
      :http-request="customUpload"
      :limit="isMultiple ? undefined : 1"
      :list-type="listType"
      :multiple="isMultiple"
      :on-error="handleError"
      :on-exceed="handleExceed"
      :on-preview="handlePreview"
      :on-remove="handleRemove"
      name="file"
    >
      <el-button
        v-if="listType === 'text' && (isMultiple || !hasFile)"
        :disabled="disabled"
        size="small"
        type="primary"
      >
        选择文件
      </el-button>
      <i
        v-else-if="listType === 'picture-card'"
        class="el-icon-plus"
      />
    </el-upload>

    <ul
      v-if="failedUploads.length"
      class="file-upload__failures"
      aria-label="上传失败的文件"
    >
      <li
        v-for="upload in failedUploads"
        :key="upload.uid"
      >
        <span>{{ upload.name }}：{{ upload.error }}</span>
        <el-button
          :disabled="disabled"
          size="mini"
          type="text"
          @click="retryUpload(upload)"
        >
          重试
        </el-button>
      </li>
    </ul>

    <el-dialog
      :title="previewType === 'image' ? '图片预览' : '视频预览'"
      :visible.sync="previewVisible"
      append-to-body
      class="file-upload__preview"
      width="60%"
      @close="clearPreview"
    >
      <video
        v-if="previewVisible && previewType === 'video'"
        :src="previewURL"
        class="file-upload__preview-media"
        controls
      />
      <img
        v-else-if="previewVisible && previewType === 'image'"
        :src="previewURL"
        alt="文件预览"
        class="file-upload__preview-media"
      >
    </el-dialog>
  </div>
</template>

<script>
import { uploadAsset } from '@/api/assets'

const DEFAULT_MAX_SIZE_BYTES = 64 * 1024 * 1024

function uploadUID(file) {
  if (file?.uid !== undefined && file?.uid !== null) {
    return String(file.uid)
  }
  return `upload-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

function errorMessage(error) {
  return error?.response?.data?.message || error?.message || '未知错误'
}

function isAbort(error, signal) {
  return (
    signal.aborted ||
    error?.code === 'ERR_CANCELED' ||
    error?.name === 'AbortError' ||
    error?.name === 'CanceledError'
  )
}

export function fileWarnings(file, uploadType, maxSizeBytes) {
  if (!(file instanceof Blob)) {
    return ['没有可上传的文件']
  }
  const warnings = []
  if (file.size === 0) {
    warnings.push('文件内容为空')
  } else if (maxSizeBytes > 0 && file.size > maxSizeBytes) {
    warnings.push(`文件超过 ${Math.ceil(maxSizeBytes / 1024 / 1024)} MiB`)
  }
  if (
    uploadType === 'Image' &&
    file.type &&
    !file.type.toLowerCase().startsWith('image/')
  ) {
    warnings.push('文件类型不是图片')
  }
  if (
    uploadType === 'Video' &&
    file.type &&
    !file.type.toLowerCase().startsWith('video/')
  ) {
    warnings.push('文件类型不是视频')
  }
  return warnings
}

export default {
  name: 'FileUpload',

  props: {
    value: {
      type: [String, Array],
      default: '',
    },
    environmentId: {
      type: [Number, String],
      required: true,
    },
    projectId: {
      type: [Number, String],
      required: true,
    },
    uploadType: {
      type: String,
      default: 'Image',
      validator: (value) => ['Image', 'Video', 'File'].includes(value),
    },
    isArray: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    maxSizeBytes: {
      type: Number,
      default: DEFAULT_MAX_SIZE_BYTES,
      validator: (value) => Number.isFinite(value) && value >= 0,
    },
  },

  data() {
    return {
      localURLs: [],
      uploadAttempts: [],
      urlNames: {},
      previewVisible: false,
      previewURL: '',
      previewType: '',
      destroying: false,
    }
  },

  computed: {
    currentValue() {
      return this.$attrs.field?.value ?? this.value
    },
    isMultiple() {
      return this.isArray
    },
    currentURLs() {
      if (!this.isMultiple) {
        return typeof this.currentValue === 'string' && this.currentValue
          ? [this.currentValue]
          : []
      }
      return this.localURLs.length
        ? this.localURLs
        : Array.isArray(this.currentValue)
          ? this.currentValue
          : []
    },
    hasFile() {
      return this.currentURLs.length > 0
    },
    acceptType() {
      if (this.uploadType === 'Image') {
        return 'image/*'
      }
      if (this.uploadType === 'Video') {
        return 'video/*'
      }
      return '*/*'
    },
    listType() {
      return this.uploadType === 'Image' ? 'picture-card' : 'text'
    },
    failedUploads() {
      return this.uploadAttempts.filter((upload) => upload.status === 'fail')
    },
    elementFileList() {
      const completed = this.currentURLs.map((url, index) => ({
        uid: `asset-${index}-${url}`,
        name: this.urlNames[url] || this.filenameFromURL(url) || `文件${index + 1}`,
        url,
        status: 'success',
        percentage: 100,
      }))
      const completedURLs = new Set(completed.map((file) => file.url))
      const attempts = this.uploadAttempts
        .filter((upload) => !upload.url || !completedURLs.has(upload.url))
        .map((upload) => ({
          uid: upload.uid,
          name: upload.name,
          url: upload.url,
          status: upload.status,
          percentage: upload.percentage,
        }))
      return [...attempts, ...completed]
    },
  },

  watch: {
    environmentId(environmentId, previousId) {
      if (String(environmentId) !== String(previousId)) {
        this.abortPendingUploads()
      }
    },
    projectId(projectId, previousId) {
      if (String(projectId) !== String(previousId)) {
        this.abortPendingUploads()
      }
    },
    currentValue: {
      immediate: true,
      deep: true,
      handler(value) {
        if (!this.isMultiple) {
          this.localURLs = []
          return
        }
        if (!Array.isArray(value)) {
          this.localURLs = []
          return
        }
        const unchanged =
          value.length === this.localURLs.length &&
          value.every((url, index) => url === this.localURLs[index])
        if (!unchanged) {
          this.localURLs = [...value]
        }
      },
    },
  },

  beforeDestroy() {
    this.destroying = true
    this.abortPendingUploads()
  },

  methods: {
    abortPendingUploads() {
      this.uploadAttempts.forEach((upload) => upload.controller.abort())
      this.uploadAttempts = []
    },
    filenameFromURL(url) {
      if (!url) {
        return ''
      }
      try {
        const base = typeof window === 'undefined' ? 'http://localhost' : window.location.origin
        return new URL(url, base).pathname.split('/').pop() || ''
      } catch {
        return ''
      }
    },
    customUpload(options) {
      return this.startUpload(options)
    },
    startUpload(options) {
      const file = options?.file
      const warnings = fileWarnings(file, this.uploadType, this.maxSizeBytes)
      if (warnings.length) {
        this.$message.warning(`${warnings.join('；')}。后端仍会执行最终校验。`)
      }
      if (!(file instanceof Blob)) {
        const error = new TypeError('没有可上传的文件')
        options?.onError?.(error, file)
        return { abort() {} }
      }

      const uid = uploadUID(file)
      const controller = new AbortController()
      const existing = this.uploadAttempts.find((upload) => upload.uid === uid)
      const upload = existing || {
        uid,
        file,
        name: file.name || '文件',
        url: '',
        status: 'uploading',
        percentage: 0,
        error: '',
        controller,
        callbacks: {},
      }
      Object.assign(upload, {
        file,
        status: 'uploading',
        percentage: 0,
        error: '',
        controller,
        callbacks: {
          onError: options?.onError,
          onProgress: options?.onProgress,
          onSuccess: options?.onSuccess,
        },
      })
      if (!existing) {
        this.uploadAttempts.push(upload)
      }

      uploadAsset(this.environmentId, this.projectId, file, {
        signal: controller.signal,
        onUploadProgress: (event) => {
          if (this.destroying || controller.signal.aborted) {
            return
          }
          const total = Number(event.total) > 0 ? Number(event.total) : file.size
          const loaded = Math.min(Number(event.loaded) || 0, total)
          const percentage = total > 0 ? Math.round((loaded / total) * 100) : 0
          upload.percentage = percentage
          upload.callbacks.onProgress?.({ percent: percentage }, file)
        },
      })
        .then((asset) => {
          if (this.destroying || controller.signal.aborted) {
            return
          }
          upload.url = asset.url
          this.commitURL(asset.url, file.name)
          this.removeAttempt(uid)
          upload.callbacks.onSuccess?.({ asset }, file)
          this.$emit('uploaded', asset)
        })
        .catch((error) => {
          if (this.destroying || isAbort(error, controller.signal)) {
            return
          }
          upload.status = 'fail'
          upload.error = errorMessage(error)
          upload.callbacks.onError?.(error, file)
        })

      return {
        abort: () => controller.abort(),
      }
    },
    retryUpload(upload) {
      return this.startUpload({
        file: upload.file,
        ...upload.callbacks,
      })
    },
    removeAttempt(uid) {
      this.uploadAttempts = this.uploadAttempts.filter(
        (upload) => upload.uid !== uid,
      )
    },
    handleError(error) {
      this.$message.error(`上传失败：${errorMessage(error)}`)
    },
    handleExceed() {
      this.$message.warning(
        this.isMultiple ? '文件数量超出限制' : '只能上传一个文件，请先删除现有文件',
      )
    },
    handlePreview(file) {
      if (this.uploadType !== 'Image' && this.uploadType !== 'Video') {
        return
      }
      if (!file?.url) {
        return
      }
      this.previewURL = file.url
      this.previewType = this.uploadType === 'Image' ? 'image' : 'video'
      this.previewVisible = true
    },
    clearPreview() {
      this.previewVisible = false
      this.previewURL = ''
      this.previewType = ''
    },
    handleRemove(file) {
      const attempt = this.uploadAttempts.find(
        (upload) => upload.uid === String(file?.uid),
      )
      if (attempt) {
        attempt.controller.abort()
        this.removeAttempt(attempt.uid)
      }
      if (file?.url) {
        this.removeURL(file.url)
      }
    },
    removeURL(url) {
      if (!this.isMultiple) {
        this.changeValue('')
        return
      }
      this.localURLs = this.currentURLs.filter((value) => value !== url)
      this.changeValue([...this.localURLs])
    },
    commitURL(url, name) {
      if (name) {
        this.$set(this.urlNames, url, name)
      }
      if (!this.isMultiple) {
        this.changeValue(url)
        return
      }
      const urls = this.currentURLs.includes(url)
        ? [...this.currentURLs]
        : [...this.currentURLs, url]
      this.localURLs = urls
      this.changeValue([...urls])
    },
    changeValue(value) {
      if (this.$attrs.field?.onChange) {
        this.$attrs.field.onChange(value)
        return
      }
      this.$emit('input', value)
      this.$emit('change', value)
    },
  },
}
</script>

<style lang="scss" scoped>
.file-upload {
  ::v-deep .el-upload__input {
    display: none;
  }
}

.file-upload--single-complete {
  ::v-deep .el-upload--picture-card,
  ::v-deep .el-upload--text {
    display: none;
  }
}

.file-upload__failures {
  margin: 8px 0 0;
  padding: 0;
  color: #f56c6c;
  font-size: 13px;
  list-style: none;

  li {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.file-upload__preview-media {
  display: block;
  width: 100%;
  max-width: 800px;
  max-height: 60vh;
  margin: 0 auto;
  object-fit: contain;
}
</style>
