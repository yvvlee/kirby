// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest'

const managementClient = vi.hoisted(() => ({
  post: vi.fn(),
}))
const objectTransport = vi.hoisted(() => {
  const put = vi.fn()
  return {
    create: vi.fn(() => ({ put })),
    put,
  }
})

vi.mock('./client', () => ({ default: managementClient }))
vi.mock('axios', () => ({
  default: { create: objectTransport.create },
}))

import { uploadAsset } from './assets'

const ticket = {
  objectKey: 'environments/11/projects/7/assets/id.png',
  uploadUrl: 'https://objects.example.com/kirby/id.png?signature=value',
  headers: {
    'Content-Type': 'image/png',
    'X-Amz-Meta-Kirby-Declared-Size': '4',
  },
  expiresAt: '2026-08-17T12:00:00Z',
}

function file() {
  return new File(['data'], 'icon.png', { type: 'image/png' })
}

beforeEach(() => {
  managementClient.post.mockReset()
  objectTransport.put.mockReset()
})

describe('asset direct upload contract', () => {
  it('按 presign、无凭据直传、complete 的顺序上传', async () => {
    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({
        data: {
          asset: {
            objectKey: ticket.objectKey,
            url: 'https://cdn.example.com/kirby/id.png',
            contentType: 'image/png',
            size: '4',
          },
        },
      })
    objectTransport.put.mockImplementation((_url, _file, config) => {
      config.onUploadProgress({ loaded: 2, total: 4 })
      return Promise.resolve({ status: 200 })
    })
    const progress = vi.fn()
    const selectedFile = file()
    const controller = new AbortController()

    await expect(
      uploadAsset(11, 7, selectedFile, {
        onUploadProgress: progress,
        signal: controller.signal,
      }),
    ).resolves.toMatchObject({
      objectKey: ticket.objectKey,
      url: 'https://cdn.example.com/kirby/id.png',
    })

    expect(managementClient.post).toHaveBeenNthCalledWith(
      1,
      '/admin/environments/11/projects/7/assets/presign',
      {
        environment_id: 11,
        project_id: 7,
        filename: 'icon.png',
        content_type: 'image/png',
        size: 4,
      },
      { signal: controller.signal },
    )
    expect(objectTransport.put).toHaveBeenCalledWith(
      ticket.uploadUrl,
      selectedFile,
      expect.objectContaining({
        headers: ticket.headers,
        withCredentials: false,
      }),
    )
    const uploadConfig = objectTransport.put.mock.calls[0][2]
    expect(
      Object.keys(uploadConfig.headers).map((name) => name.toLowerCase()),
    ).not.toEqual(
      expect.arrayContaining([
        'authorization',
        'cookie',
        'x-kirby-api-key',
      ]),
    )
    expect(progress).toHaveBeenCalledWith({ loaded: 2, total: 4 })
    expect(managementClient.post).toHaveBeenNthCalledWith(
      2,
      '/admin/environments/11/projects/7/assets/complete',
      {
        environment_id: 11,
        project_id: 7,
        object_key: ticket.objectKey,
      },
      { signal: controller.signal },
    )
  })

  it('对象存储客户端关闭 Cookie 和 XSRF 凭据', () => {
    expect(objectTransport.create).toHaveBeenCalledWith({
      withCredentials: false,
      withXSRFToken: false,
      xsrfCookieName: null,
      xsrfHeaderName: null,
    })
  })

  it.each(['Authorization', 'Cookie', 'X-Kirby-API-Key'])(
    '拒绝后端返回的敏感直传请求头 %s',
    async (header) => {
      managementClient.post.mockResolvedValueOnce({
        data: {
          ...ticket,
          headers: { [header]: 'secret' },
        },
      })

      await expect(uploadAsset(11, 7, file())).rejects.toThrow('is forbidden')
      expect(objectTransport.put).not.toHaveBeenCalled()
      expect(managementClient.post).toHaveBeenCalledTimes(1)
    },
  )

  it('拒绝非 HTTP 的直传地址和完成地址', async () => {
    managementClient.post.mockResolvedValueOnce({
      data: { ...ticket, uploadUrl: 'javascript:alert(1)' },
    })
    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'asset uploadUrl must be an HTTP URL',
    )

    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({
        data: {
          asset: {
            objectKey: ticket.objectKey,
            url: 'data:text/plain,secret',
            contentType: 'image/png',
            size: '4',
          },
        },
      })
    objectTransport.put.mockResolvedValue({ status: 200 })
    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'asset url must be an HTTP URL',
    )
  })

  it('拒绝 complete 返回其他对象的地址', async () => {
    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({
        data: {
          asset: {
            objectKey: 'environments/99/projects/7/assets/other.png',
            url: '/api/assets/objects/other.png',
            contentType: 'image/png',
            size: '4',
          },
        },
      })
    objectTransport.put.mockResolvedValue({ status: 200 })

    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'mismatched objectKey',
    )
  })
})
