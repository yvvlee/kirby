import { App as AntdApp } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { uploadAsset, type UploadedAsset } from '@/api/assets'
import FileUpload from './FileUpload'

vi.mock('@/api/assets', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/api/assets')>()
  return { ...original, uploadAsset: vi.fn() }
})

const uploadAssetMock = vi.mocked(uploadAsset)
const asset: UploadedAsset = {
  objectKey: 'environments/11/projects/7/assets/123e4567-e89b-42d3-a456-426614174000.txt',
  url: '/assets/123e4567-e89b-42d3-a456-426614174000.txt',
  contentType: 'text/plain',
  size: 4,
}

function uploadInput(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')
  if (!input) throw new Error('上传输入框没有渲染')
  return input
}

describe('FileUpload', () => {
  beforeEach(() => uploadAssetMock.mockReset())

  it('reports progress and writes the completed URL', async () => {
    const onChange = vi.fn()
    uploadAssetMock.mockImplementationOnce(async (_environmentId, _projectId, _file, options) => {
      if (!options) throw new Error('上传选项缺失')
      options.onUploadProgress?.({
        loaded: 4,
        total: 4,
        bytes: 4,
        lengthComputable: true,
      })
      return asset
    })
    const { container } = render(
      <StrictMode>
        <AntdApp>
          <FileUpload environmentId={11} projectId={7} uploadType="File" onChange={onChange} />
        </AntdApp>
      </StrictMode>,
    )

    fireEvent.change(uploadInput(container), {
      target: { files: [new File(['data'], 'report.txt', { type: 'text/plain' })] },
    })

    await waitFor(() => expect(onChange).toHaveBeenCalledWith(asset.url))
    expect(uploadAssetMock).toHaveBeenCalledWith(
      11,
      7,
      expect.any(File),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
  })

  it('keeps a failed upload visible and retries it', async () => {
    uploadAssetMock
      .mockRejectedValueOnce(new Error('网络失败'))
      .mockResolvedValueOnce(asset)
    const onChange = vi.fn()
    const user = userEvent.setup()
    const { container } = render(
      <AntdApp>
        <FileUpload environmentId={11} projectId={7} uploadType="File" onChange={onChange} />
      </AntdApp>,
    )

    fireEvent.change(uploadInput(container), {
      target: { files: [new File(['data'], 'retry.txt', { type: 'text/plain' })] },
    })
    expect(await screen.findByText(/retry\.txt：网络失败/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /重试/ }))

    await waitFor(() => expect(onChange).toHaveBeenCalledWith(asset.url))
    expect(uploadAssetMock).toHaveBeenCalledTimes(2)
  })

  it('aborts an active upload on unmount', async () => {
    let uploadSignal: AbortSignal | undefined
    uploadAssetMock.mockImplementationOnce((_environmentId, _projectId, _file, options) => {
      if (!options) throw new Error('上传选项缺失')
      uploadSignal = options.signal
      return new Promise((_resolve, reject) => {
        options.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })
    })
    const { container, unmount } = render(
      <AntdApp>
        <FileUpload environmentId={11} projectId={7} uploadType="File" />
      </AntdApp>,
    )

    fireEvent.change(uploadInput(container), {
      target: { files: [new File(['data'], 'pending.txt')] },
    })
    await waitFor(() => expect(uploadSignal).toBeDefined())
    unmount()

    expect(uploadSignal?.aborted).toBe(true)
  })

  it('aborts old uploads when the environment scope changes', async () => {
    let uploadSignal: AbortSignal | undefined
    uploadAssetMock.mockImplementationOnce((_environmentId, _projectId, _file, options) => {
      if (!options) throw new Error('上传选项缺失')
      uploadSignal = options.signal
      return new Promise((_resolve, reject) => {
        options.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })
    })
    const { container, rerender } = render(
      <AntdApp>
        <FileUpload environmentId={11} projectId={7} uploadType="File" />
      </AntdApp>,
    )

    fireEvent.change(uploadInput(container), {
      target: { files: [new File(['data'], 'pending.txt')] },
    })
    await waitFor(() => expect(uploadSignal).toBeDefined())
    rerender(
      <AntdApp>
        <FileUpload environmentId={12} projectId={7} uploadType="File" />
      </AntdApp>,
    )

    expect(uploadSignal?.aborted).toBe(true)
  })
})
