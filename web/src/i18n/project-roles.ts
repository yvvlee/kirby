import { t } from './index'

const projectRoleLabels: Record<string, string> = {
  OWNER: '所有者',
  MAINTAINER: '维护者',
  MEMBER: '成员',
  GUEST: '访客',
}

/**
 * Returns a localized label for a fixed project role while preserving unknown codes.
 * Parameters: role is the role code returned by the API.
 * Returns: the localized role label, or the original code if it is unknown.
 * Throws: none.
 */
export function projectRoleLabel(role: string): string {
  const source = projectRoleLabels[role]
  return source ? t(source) : role
}
