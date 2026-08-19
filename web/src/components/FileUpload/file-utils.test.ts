import { describe, expect, it } from 'vitest'

import { fileWarnings, filenameFromUrl, isAbortError } from './file-utils'

describe('file upload utilities', () => {
  it('reports empty, oversized, and mismatched files without blocking them', () => {
    expect(fileWarnings(new File([], 'empty.txt'), 'Image', 1)).toEqual([
      '文件内容为空',
    ])
    expect(
      fileWarnings(new File(['large'], 'clip.txt', { type: 'text/plain' }), 'Video', 2),
    ).toEqual(['文件超过 1 MiB', '文件类型不是视频'])
  })

  it('extracts names from relative and absolute URLs', () => {
    expect(filenameFromUrl('/assets/report.json?version=2')).toBe('report.json')
    expect(filenameFromUrl('https://cdn.example.com/a/photo.png')).toBe('photo.png')
  })

  it('recognizes signal and transport cancellation errors', () => {
    const controller = new AbortController()
    controller.abort()
    expect(isAbortError(new Error('anything'), controller.signal)).toBe(true)
    expect(isAbortError({ code: 'ERR_CANCELED' }, new AbortController().signal)).toBe(true)
    expect(isAbortError(new Error('network'), new AbortController().signal)).toBe(false)
  })
})
