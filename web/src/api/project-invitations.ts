import client from './client'
import type { Identifier } from './types'

export type ProjectInvitation = {
  project_id: Identifier
  role: string
  expires_at: string
  token: string
}

/** Create a single-use project invitation; only the response contains the token. */
export async function createProjectInvitation(projectId: Identifier, role: string): Promise<ProjectInvitation> {
  const { data } = await client.post<ProjectInvitation>(`/admin/projects/${projectId}/invitations`, {
    project_id: projectId,
    role,
  })
  return data
}

/** Revoke a previously generated invitation link. */
export async function revokeProjectInvitation(token: string): Promise<void> {
  await client.delete(`/invitations/${encodeURIComponent(token)}`)
}
