import client from './client'
import type { ApiListReply, ApiObject, Identifier, User } from './types'

export type ProjectMember = ApiObject & { user: User; role: string }
const path = (projectId: Identifier) => `/admin/projects/${projectId}/members`
export async function listProjectMembers(projectId: Identifier) { const { data } = await client.get<ApiListReply<ProjectMember>>(path(projectId)); return data }
export async function addProjectMember(projectId: Identifier, userId: Identifier, role: string) { const { data } = await client.post<ProjectMember>(path(projectId), { project_id: projectId, user_id: userId, role }); return data }
export async function updateProjectMemberRole(projectId: Identifier, userId: Identifier, role: string) { const { data } = await client.put<ProjectMember>(`${path(projectId)}/${userId}`, { project_id: projectId, user_id: userId, role }); return data }
export async function removeProjectMember(projectId: Identifier, userId: Identifier) { return client.delete(`${path(projectId)}/${userId}`) }

export async function searchProjectUsers(projectId: Identifier, keyword: string) {
  const { data } = await client.get<ApiListReply<User>>(`/admin/projects/${projectId}/users`, { params: { keyword } })
  return data
}
