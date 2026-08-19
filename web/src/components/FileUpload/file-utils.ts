export type UploadType = 'Image' | 'Video' | 'File'

export function fileWarnings(
  file: unknown,
  uploadType: UploadType,
  maxSizeBytes: number,
): string[] {
  if (!(file instanceof Blob)) return ['没有可上传的文件']
  const warnings: string[] = []
  if (file.size === 0) {
    warnings.push('文件内容为空')
  } else if (maxSizeBytes > 0 && file.size > maxSizeBytes) {
    warnings.push(`文件超过 ${Math.ceil(maxSizeBytes / 1024 / 1024)} MiB`)
  }
  if (uploadType === 'Image' && file.type && !file.type.toLowerCase().startsWith('image/')) {
    warnings.push('文件类型不是图片')
  }
  if (uploadType === 'Video' && file.type && !file.type.toLowerCase().startsWith('video/')) {
    warnings.push('文件类型不是视频')
  }
  return warnings
}

export function filenameFromUrl(url: string): string {
  if (!url) return ''
  try {
    return new URL(url, window.location.origin).pathname.split('/').pop() ?? ''
  } catch {
    return ''
  }
}

export function isAbortError(error: unknown, signal: AbortSignal): boolean {
  if (signal.aborted) return true
  if (typeof error !== 'object' || error === null) return false
  return (
    ('code' in error && error.code === 'ERR_CANCELED') ||
    ('name' in error && ['AbortError', 'CanceledError'].includes(String(error.name)))
  )
}
