// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest'

const managementClient = vi.hoisted(() => ({
  post: vi.fn(),
}))
const objectTransport = vi.hoisted(() => {
  const post = vi.fn()
  const put = vi.fn()
  return {
    create: vi.fn(() => ({ post, put })),
    post,
    put,
  }
})

vi.mock('./client', () => ({ default: managementClient }))
vi.mock('axios', () => ({
  default: { create: objectTransport.create },
}))

import { uploadAsset } from './assets'

const uploadKey =
  'uploads/environments/11/projects/7/assets/123e4567-e89b-42d3-a456-426614174000.png'
const finalKey =
  'environments/11/projects/7/assets/223e4567-e89b-42d3-a456-426614174000.png'
const ticket = {
  objectKey: uploadKey,
  uploadUrl: 'https://objects.example.com/kirby',
  uploadMethod: 'POST',
  headers: {},
  formFields: {
    key: uploadKey,
    policy: 'signed-storage-policy',
    'x-amz-credential': 'storage-access/20260817/us-east-1/s3/aws4_request',
    'x-amz-signature': 'storage-signature',
  },
  expiresAt: '2026-08-17T12:00:00Z',
}

function file() {
  return new File(['data'], 'icon.png', { type: 'image/png' })
}

function completedAsset(objectKey = finalKey) {
  return {
    data: {
      asset: {
        objectKey,
        url: `https://cdn.example.com/kirby/${objectKey}`,
        contentType: 'image/png',
        size: '4',
      },
    },
  }
}

beforeEach(() => {
  managementClient.post.mockReset()
  objectTransport.post.mockReset()
  objectTransport.put.mockReset()
})

describe('asset direct upload contract', () => {
  it('按 presign、S3 POST policy 直传、complete 的顺序上传', async () => {
    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce(completedAsset())
    objectTransport.post.mockImplementation((_url, _form, config) => {
      config.onUploadProgress({ loaded: 2, total: 4 })
      return Promise.resolve({ status: 204 })
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
      objectKey: finalKey,
      url: `https://cdn.example.com/kirby/${finalKey}`,
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
    expect(objectTransport.post).toHaveBeenCalledWith(
      ticket.uploadUrl,
      expect.any(FormData),
      expect.objectContaining({
        headers: {},
        withCredentials: false,
      }),
    )
    const [formEntries, uploadConfig] = [
      [...objectTransport.post.mock.calls[0][1].entries()],
      objectTransport.post.mock.calls[0][2],
    ]
    expect(formEntries.slice(0, -1)).toEqual(Object.entries(ticket.formFields))
    expect(formEntries.at(-1)[0]).toBe('file')
    expect(formEntries.at(-1)[1]).toBeInstanceOf(File)
    expect(
      Object.keys(uploadConfig.headers).map((name) => name.toLowerCase()),
    ).not.toEqual(
      expect.arrayContaining(['authorization', 'cookie', 'x-kirby-api-key']),
    )
    expect(progress).toHaveBeenCalledWith({ loaded: 2, total: 4 })
    expect(managementClient.post).toHaveBeenNthCalledWith(
      2,
      '/admin/environments/11/projects/7/assets/complete',
      {
        environment_id: 11,
        project_id: 7,
        object_key: uploadKey,
      },
      { signal: controller.signal },
    )
  })

  it('保留无表单字段的本地 PUT 直传', async () => {
    const localKey = finalKey
    managementClient.post
      .mockResolvedValueOnce({
        data: {
          objectKey: localKey,
          uploadUrl: '/api/assets/upload?token=signed',
          uploadMethod: 'PUT',
          headers: { 'Content-Type': 'image/png' },
          formFields: {},
          expiresAt: ticket.expiresAt,
        },
      })
      .mockResolvedValueOnce(completedAsset(localKey))
    objectTransport.put.mockResolvedValue({ status: 204 })
    const selectedFile = file()

    await uploadAsset(11, 7, selectedFile)

    expect(objectTransport.put).toHaveBeenCalledWith(
      '/api/assets/upload?token=signed',
      selectedFile,
      expect.objectContaining({
        headers: { 'Content-Type': 'image/png' },
        withCredentials: false,
      }),
    )
    expect(objectTransport.post).not.toHaveBeenCalled()
  })

  it('POST 只能使用临时键，PUT 只能使用最终键', async () => {
    managementClient.post.mockResolvedValueOnce({
      data: {
        ...ticket,
        objectKey: finalKey,
        formFields: { ...ticket.formFields, key: finalKey },
      },
    })
    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'POST upload requires a temporary objectKey',
    )

    managementClient.post.mockResolvedValueOnce({
      data: {
        ...ticket,
        uploadMethod: 'PUT',
        headers: { 'Content-Type': 'image/png' },
        formFields: {},
      },
    })
    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'PUT upload requires a final objectKey',
    )
    expect(objectTransport.post).not.toHaveBeenCalled()
    expect(objectTransport.put).not.toHaveBeenCalled()
  })

  it('POST 表单 key 必须与票据 objectKey 完全相同', async () => {
    managementClient.post.mockResolvedValueOnce({
      data: {
        ...ticket,
        formFields: {
          ...ticket.formFields,
          key: 'uploads/environments/11/projects/7/assets/323e4567-e89b-42d3-a456-426614174000.png',
        },
      },
    })

    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'form key does not match objectKey',
    )
    expect(objectTransport.post).not.toHaveBeenCalled()
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
        data: { ...ticket, headers: { [header]: 'management-secret' } },
      })

      await expect(uploadAsset(11, 7, file())).rejects.toThrow('is forbidden')
      expect(objectTransport.post).not.toHaveBeenCalled()
      expect(objectTransport.put).not.toHaveBeenCalled()
    },
  )

  it.each(['Authorization', 'Cookie', 'X-Kirby-API-Key', 'file'])(
    '拒绝后端返回的敏感或保留表单字段 %s',
    async (field) => {
      managementClient.post.mockResolvedValueOnce({
        data: {
          ...ticket,
          formFields: { ...ticket.formFields, [field]: 'management-secret' },
        },
      })

      await expect(uploadAsset(11, 7, file())).rejects.toThrow('is forbidden')
      expect(objectTransport.post).not.toHaveBeenCalled()
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
            objectKey: finalKey,
            url: 'data:text/plain,secret',
            contentType: 'image/png',
            size: '4',
          },
        },
      })
    objectTransport.post.mockResolvedValue({ status: 204 })
    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'asset url must be an HTTP URL',
    )
  })

  it('接受服务端新最终键，但拒绝跨环境、跨项目或临时键', async () => {
    for (const returnedKey of [
      'environments/99/projects/7/assets/223e4567-e89b-42d3-a456-426614174000.png',
      'environments/11/projects/99/assets/223e4567-e89b-42d3-a456-426614174000.png',
      uploadKey,
    ]) {
      managementClient.post
        .mockResolvedValueOnce({ data: ticket })
        .mockResolvedValueOnce(completedAsset(returnedKey))
      objectTransport.post.mockResolvedValueOnce({ status: 204 })

      await expect(uploadAsset(11, 7, file())).rejects.toThrow(
        'mismatched environment or project scope',
      )
    }
  })
})
