import { beforeEach, describe, expect, it, vi } from 'vitest'

const managementClient = vi.hoisted(() => ({ post: vi.fn() }))
const objectTransport = vi.hoisted(() => {
  const post = vi.fn()
  const put = vi.fn()
  return { create: vi.fn(() => ({ post, put })), post, put }
})

vi.mock('./client', () => ({ default: managementClient }))
vi.mock('axios', () => ({ default: { create: objectTransport.create } }))

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
    'x-amz-signature': 'storage-signature',
  },
  expiresAt: '2026-08-19T12:00:00Z',
}

function file(): File {
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
  it('uploads through presign, object storage, and complete', async () => {
    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce(completedAsset())
    objectTransport.post.mockResolvedValue({ status: 204 })
    const selectedFile = file()
    const controller = new AbortController()

    await expect(
      uploadAsset(11, 7, selectedFile, { signal: controller.signal }),
    ).resolves.toMatchObject({ objectKey: finalKey })

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
      expect.objectContaining({ headers: {}, withCredentials: false }),
    )
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

  it('supports local PUT tickets without form fields', async () => {
    managementClient.post
      .mockResolvedValueOnce({
        data: {
          ...ticket,
          objectKey: finalKey,
          uploadUrl: '/api/assets/upload?token=signed',
          uploadMethod: 'PUT',
          headers: { 'Content-Type': 'image/png' },
          formFields: {},
        },
      })
      .mockResolvedValueOnce(completedAsset())
    objectTransport.put.mockResolvedValue({ status: 204 })

    await uploadAsset(11, 7, file())

    expect(objectTransport.put).toHaveBeenCalledWith(
      '/api/assets/upload?token=signed',
      expect.any(File),
      expect.objectContaining({
        headers: { 'Content-Type': 'image/png' },
        withCredentials: false,
      }),
    )
  })

  it.each(['Authorization', 'Cookie', 'X-Kirby-API-Key'])(
    'rejects sensitive upload header %s',
    async (header) => {
      managementClient.post.mockResolvedValueOnce({
        data: { ...ticket, headers: { [header]: 'management-secret' } },
      })

      await expect(uploadAsset(11, 7, file())).rejects.toThrow('is forbidden')
      expect(objectTransport.post).not.toHaveBeenCalled()
    },
  )

  it('rejects non-HTTP upload and completion URLs', async () => {
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

  it('rejects a completed object from another scope', async () => {
    managementClient.post
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce(
        completedAsset(
          'environments/99/projects/7/assets/223e4567-e89b-42d3-a456-426614174000.png',
        ),
      )
    objectTransport.post.mockResolvedValue({ status: 204 })

    await expect(uploadAsset(11, 7, file())).rejects.toThrow(
      'mismatched environment or project scope',
    )
  })

  it('creates the storage client without credentials or XSRF', () => {
    expect(objectTransport.create).toHaveBeenCalledWith({
      withCredentials: false,
      withXSRFToken: false,
      xsrfCookieName: null,
      xsrfHeaderName: null,
    })
  })
})
