import { PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons'
import { App, Button, List, Modal, Upload } from 'antd'
import type { UploadFile, UploadProps } from 'antd/es/upload/interface'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { uploadAsset, type UploadedAsset } from '@/api/assets'
import { getApiErrorMessage } from '@/api/errors'
import type { Identifier } from '@/api/types'
import {
  filenameFromUrl,
  fileWarnings,
  isAbortError,
  type UploadType,
} from './file-utils'

const DEFAULT_MAX_SIZE_BYTES = 64 * 1024 * 1024

type UploadCallbacks = Pick<
  Parameters<NonNullable<UploadProps['customRequest']>>[0],
  'onError' | 'onProgress' | 'onSuccess'
>

type UploadAttempt = {
  uid: string
  file: File
  name: string
  status: 'uploading' | 'error'
  percentage: number
  error: string
  controller: AbortController
  callbacks: UploadCallbacks
}

export type FileUploadProps = {
  value?: string | string[]
  environmentId: Identifier
  projectId: Identifier
  uploadType?: UploadType
  isArray?: boolean
  disabled?: boolean
  maxSizeBytes?: number
  onChange?: (value: string | string[]) => void
  onUploaded?: (asset: UploadedAsset) => void
}

export default function FileUpload({
  value = '',
  environmentId,
  projectId,
  uploadType = 'Image',
  isArray = false,
  disabled = false,
  maxSizeBytes = DEFAULT_MAX_SIZE_BYTES,
  onChange,
  onUploaded,
}: FileUploadProps) {
  const { message } = App.useApp()
  const [attempts, setAttempts] = useState<UploadAttempt[]>([])
  const [preview, setPreview] = useState<{ url: string; type: 'image' | 'video' } | null>(null)
  const names = useRef(new Map<string, string>())
  const attemptsRef = useRef(attempts)
  const valueRef = useRef(value)
  const destroyed = useRef(false)

  useEffect(() => { attemptsRef.current = attempts }, [attempts])
  useEffect(() => { valueRef.current = value }, [value])

  const abortPending = useCallback(() => {
    attemptsRef.current.forEach((attempt) => attempt.controller.abort())
    attemptsRef.current = []
    setAttempts([])
  }, [])

  useEffect(() => abortPending, [abortPending, environmentId, projectId])
  useEffect(() => {
    destroyed.current = false
    return () => {
      destroyed.current = true
      abortPending()
    }
  }, [abortPending])

  const currentUrls = useMemo(() => {
    if (isArray) return Array.isArray(value) ? value : []
    return typeof value === 'string' && value ? [value] : []
  }, [isArray, value])

  const commitUrl = useCallback((url: string, name: string) => {
    if (name) names.current.set(url, name)
    if (!isArray) {
      onChange?.(url)
      return
    }
    const current = Array.isArray(valueRef.current) ? valueRef.current : []
    onChange?.(current.includes(url) ? [...current] : [...current, url])
  }, [isArray, onChange])

  const removeAttempt = useCallback((uid: string) => {
    setAttempts((current) => current.filter((attempt) => attempt.uid !== uid))
  }, [])

  const startUpload = useCallback((file: File, callbacks: UploadCallbacks) => {
    const warnings = fileWarnings(file, uploadType, maxSizeBytes)
    if (warnings.length) void message.warning(`${warnings.join('；')}。后端仍会执行最终校验。`)
    const uid = 'uid' in file ? String(file.uid) : crypto.randomUUID()
    const controller = new AbortController()
    const attempt: UploadAttempt = {
      uid,
      file,
      name: file.name || '文件',
      status: 'uploading',
      percentage: 0,
      error: '',
      controller,
      callbacks,
    }
    setAttempts((current) => [...current.filter((item) => item.uid !== uid), attempt])

    void uploadAsset(environmentId, projectId, file, {
      signal: controller.signal,
      onUploadProgress: (event) => {
        if (destroyed.current || controller.signal.aborted) return
        const total = Number(event.total) > 0 ? Number(event.total) : file.size
        const loaded = Math.min(Number(event.loaded) || 0, total)
        const percentage = total > 0 ? Math.round((loaded / total) * 100) : 0
        setAttempts((current) => current.map((item) =>
          item.uid === uid ? { ...item, percentage } : item,
        ))
        callbacks.onProgress?.({ percent: percentage }, file)
      },
    }).then((asset) => {
      if (destroyed.current || controller.signal.aborted) return
      commitUrl(asset.url, file.name)
      removeAttempt(uid)
      callbacks.onSuccess?.({ asset }, file)
      onUploaded?.(asset)
    }).catch((error: unknown) => {
      if (destroyed.current || isAbortError(error, controller.signal)) return
      const text = getApiErrorMessage(error, '未知错误')
      setAttempts((current) => current.map((item) =>
        item.uid === uid ? { ...item, status: 'error', error: text } : item,
      ))
      callbacks.onError?.(error instanceof Error ? error : new Error(text))
    })
    return { abort: () => controller.abort() }
  }, [commitUrl, environmentId, maxSizeBytes, message, onUploaded, projectId, removeAttempt, uploadType])

  const customRequest: NonNullable<UploadProps['customRequest']> = (options) => {
    if (!(options.file instanceof File)) {
      options.onError?.(new TypeError('没有可上传的文件'))
      return
    }
    return startUpload(options.file, options)
  }

  const fileList: UploadFile[] = [
    ...attempts.map((attempt) => ({
      uid: attempt.uid,
      name: attempt.name,
      status: attempt.status,
      percent: attempt.percentage,
    }) satisfies UploadFile),
    ...currentUrls.map((url, index) => ({
      uid: `asset-${index}-${url}`,
      name: names.current.get(url) || filenameFromUrl(url) || `文件${index + 1}`,
      url,
      status: 'done' as const,
      percent: 100,
    })),
  ]

  const remove = (file: UploadFile) => {
    const attempt = attemptsRef.current.find((item) => item.uid === file.uid)
    if (attempt) {
      attempt.controller.abort()
      removeAttempt(attempt.uid)
    }
    if (file.url) {
      if (isArray) onChange?.(currentUrls.filter((url) => url !== file.url))
      else onChange?.('')
    }
  }

  const showPreview = (file: UploadFile) => {
    if (!file.url || uploadType === 'File') return
    setPreview({ url: file.url, type: uploadType === 'Image' ? 'image' : 'video' })
  }

  const failed = attempts.filter((attempt) => attempt.status === 'error')
  const listType = uploadType === 'Image' ? 'picture-card' : 'text'
  const canChoose = isArray || currentUrls.length === 0

  return (
    <div className="file-upload">
      <Upload
        {...(isArray ? {} : { maxCount: 1 })}
        action="#"
        accept={uploadType === 'Image' ? 'image/*' : uploadType === 'Video' ? 'video/*' : '*/*'}
        customRequest={customRequest}
        disabled={disabled}
        fileList={fileList}
        listType={listType}
        multiple={isArray}
        onPreview={showPreview}
        onRemove={remove}
      >
        {canChoose ? (
          listType === 'picture-card' ? <PlusOutlined /> : (
            <Button disabled={disabled} icon={<UploadOutlined />}>选择文件</Button>
          )
        ) : null}
      </Upload>
      {failed.length ? (
        <List
          aria-label="上传失败的文件"
          size="small"
          dataSource={failed}
          renderItem={(attempt) => (
            <List.Item actions={[
              <Button
                key="retry"
                type="text"
                icon={<ReloadOutlined />}
                disabled={disabled}
                onClick={() => startUpload(attempt.file, attempt.callbacks)}
              >重试</Button>,
            ]}>
              {attempt.name}：{attempt.error}
            </List.Item>
          )}
        />
      ) : null}
      <Modal
        open={preview !== null}
        title={preview?.type === 'image' ? '图片预览' : '视频预览'}
        footer={null}
        onCancel={() => setPreview(null)}
      >
        {preview?.type === 'image' ? <img className="file-preview" src={preview.url} alt="文件预览" /> : null}
        {preview?.type === 'video' ? <video className="file-preview" src={preview.url} controls /> : null}
      </Modal>
    </div>
  )
}
