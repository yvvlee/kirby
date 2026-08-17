// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import Vue from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const assetAPI = vi.hoisted(() => ({
  uploadAsset: vi.fn(),
}))

vi.mock('@/api/assets', () => assetAPI)

import FileUpload, { fileWarnings } from './FileUpload.vue'

Vue.config.ignoredElements = [/^el-/]
Vue.config.productionTip = false

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function selectedFile(contents = 'data', options = {}) {
  const file = new File([contents], options.name || 'icon.png', {
    type: options.type || 'image/png',
  })
  Object.defineProperty(file, 'uid', {
    configurable: true,
    value: options.uid || 'file-1',
  })
  return file
}

function mountComponent(propsData = {}) {
  const message = {
    error: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  }
  return {
    message,
    wrapper: shallowMount(FileUpload, {
      propsData: {
        environmentId: 11,
        projectId: 7,
        ...propsData,
      },
      mocks: { $message: message },
    }),
  }
}

beforeEach(() => {
  assetAPI.uploadAsset.mockReset()
})

describe('FileUpload', () => {
  it('显示上传进度并只提交 complete 返回的 URL', async () => {
    assetAPI.uploadAsset.mockImplementation(
      (_environmentId, _projectId, _file, options) => {
        options.onUploadProgress({ loaded: 2, total: 4 })
        return Promise.resolve({
          objectKey: 'environments/11/projects/7/assets/id.png',
          url: 'https://cdn.example.com/kirby/id.png',
          contentType: 'image/png',
          size: '4',
        })
      },
    )
    const { wrapper } = mountComponent()
    const onProgress = vi.fn()
    const onSuccess = vi.fn()

    wrapper.vm.customUpload({
      file: selectedFile(),
      onProgress,
      onSuccess,
      onError: vi.fn(),
    })
    await flush()

    expect(assetAPI.uploadAsset).toHaveBeenCalledWith(
      11,
      7,
      expect.any(File),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(onProgress).toHaveBeenCalledWith(
      { percent: 50 },
      expect.any(File),
    )
    expect(wrapper.emitted('input')[0]).toEqual([
      'https://cdn.example.com/kirby/id.png',
    ])
    expect(onSuccess).toHaveBeenCalledWith(
      expect.objectContaining({
        asset: expect.objectContaining({
          url: 'https://cdn.example.com/kirby/id.png',
        }),
      }),
      expect.any(File),
    )
  })

  it('上传失败后保留文件并可以重试', async () => {
    assetAPI.uploadAsset
      .mockRejectedValueOnce(new Error('对象存储不可用'))
      .mockResolvedValueOnce({
        objectKey: 'environments/11/projects/7/assets/id.png',
        url: '/api/assets/objects/environments/11/projects/7/assets/id.png',
        contentType: 'image/png',
        size: '4',
      })
    const { wrapper } = mountComponent()
    const onError = vi.fn()
    const onSuccess = vi.fn()

    wrapper.vm.customUpload({
      file: selectedFile(),
      onProgress: vi.fn(),
      onSuccess,
      onError,
    })
    await flush()

    expect(wrapper.vm.failedUploads).toHaveLength(1)
    expect(wrapper.vm.failedUploads[0].error).toBe('对象存储不可用')
    expect(onError).toHaveBeenCalledTimes(1)

    wrapper.vm.retryUpload(wrapper.vm.failedUploads[0])
    await flush()

    expect(assetAPI.uploadAsset).toHaveBeenCalledTimes(2)
    expect(wrapper.vm.failedUploads).toHaveLength(0)
    expect(wrapper.emitted('input')[0]).toEqual([
      '/api/assets/objects/environments/11/projects/7/assets/id.png',
    ])
    expect(onSuccess).toHaveBeenCalledTimes(1)
  })

  it('卸载时中止全部未完成上传且不再回调错误', async () => {
    let uploadSignal
    assetAPI.uploadAsset.mockImplementation(
      (_environmentId, _projectId, _file, { signal }) => {
        uploadSignal = signal
        return new Promise((_resolve, reject) => {
          signal.addEventListener('abort', () => {
            const error = new Error('aborted')
            error.name = 'AbortError'
            reject(error)
          })
        })
      },
    )
    const { wrapper } = mountComponent()
    const onError = vi.fn()
    wrapper.vm.customUpload({
      file: selectedFile(),
      onError,
      onProgress: vi.fn(),
      onSuccess: vi.fn(),
    })

    wrapper.destroy()
    await flush()

    expect(uploadSignal.aborted).toBe(true)
    expect(onError).not.toHaveBeenCalled()
  })

  it('作用域变化时中止旧上传，新上传使用新环境和项目', async () => {
    const calls = []
    assetAPI.uploadAsset.mockImplementation(
      (environmentId, projectId, _file, { signal }) => {
        calls.push({ environmentId, projectId, signal })
        return new Promise((_resolve, reject) => {
          signal.addEventListener('abort', () => {
            const error = new Error('aborted')
            error.name = 'AbortError'
            reject(error)
          })
        })
      },
    )
    const { wrapper } = mountComponent()
    const options = {
      file: selectedFile(),
      onError: vi.fn(),
      onProgress: vi.fn(),
      onSuccess: vi.fn(),
    }

    wrapper.vm.customUpload(options)
    await wrapper.setProps({ environmentId: 22, projectId: 8 })
    await flush()

    expect(calls[0]).toMatchObject({ environmentId: 11, projectId: 7 })
    expect(calls[0].signal.aborted).toBe(true)
    expect(options.onError).not.toHaveBeenCalled()

    wrapper.vm.customUpload({
      ...options,
      file: selectedFile('new', { uid: 'file-2' }),
    })
    expect(calls[1]).toMatchObject({ environmentId: 22, projectId: 8 })
    wrapper.destroy()
  })

  it('类型和大小检查只提示，仍由后端判断', async () => {
    assetAPI.uploadAsset.mockResolvedValue({
      objectKey: 'environments/11/projects/7/assets/id.txt',
      url: '/api/assets/objects/environments/11/projects/7/assets/id.txt',
      contentType: 'text/plain',
      size: '4',
    })
    const { message, wrapper } = mountComponent({ maxSizeBytes: 1 })
    const file = selectedFile('data', {
      name: 'notes.txt',
      type: 'text/plain',
    })

    wrapper.vm.customUpload({
      file,
      onError: vi.fn(),
      onProgress: vi.fn(),
      onSuccess: vi.fn(),
    })
    await flush()

    expect(message.warning).toHaveBeenCalledWith(
      expect.stringContaining('后端仍会执行最终校验'),
    )
    expect(assetAPI.uploadAsset).toHaveBeenCalledTimes(1)
  })

  it('可以把多文件结果写回 Formily 字段', () => {
    const onChange = vi.fn()
    const { wrapper } = mountComponent({
      isArray: true,
      value: ['https://cdn.example.com/first.png'],
    })
    wrapper.vm.$attrs.field = {
      value: ['https://cdn.example.com/first.png'],
      onChange,
    }

    wrapper.vm.commitURL('https://cdn.example.com/second.png', 'second.png')

    expect(onChange).toHaveBeenCalledWith([
      'https://cdn.example.com/first.png',
      'https://cdn.example.com/second.png',
    ])
  })
})

describe('fileWarnings', () => {
  it('识别大小和字段类型不匹配，但不承担后端校验', () => {
    const file = selectedFile('data', {
      name: 'notes.txt',
      type: 'text/plain',
    })
    expect(fileWarnings(file, 'Image', 1)).toEqual([
      '文件超过 1 MiB',
      '文件类型不是图片',
    ])
  })
})
