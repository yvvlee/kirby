import { Alert, Button, Checkbox, Space, Table } from 'antd'
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { getApiErrorMessage } from '@/api/errors'
import { useTranslation } from '@/i18n'
import { projectRoleLabel } from '@/i18n/project-roles'
import {
  getEnvironmentPermissionMatrix,
  updateEnvironmentPermissionMatrix,
  type PermissionRole,
} from '@/api/environment-permissions'
export default function EnvironmentPermissionsPage() {
  const { t } = useTranslation()
  const points = [
    {
      key: 'READ',
      name: t('读'),
    },
    {
      key: 'WRITE',
      name: t('写'),
    },
    {
      key: 'PUBLISH',
      name: t('发布/下线'),
    },
    {
      key: 'API_KEY_MANAGE',
      name: t('API Key 管理'),
    },
  ]
  const { projectId = '', environmentId = '' } = useParams()
  const cache = useQueryClient()
  const [roles, setRoles] = useState<PermissionRole[]>([])
  const matrix = useQuery({
    queryKey: ['environment-matrix', environmentId],
    queryFn: () => getEnvironmentPermissionMatrix(environmentId),
  })
  useEffect(() => {
    if (matrix.data)
      setRoles(
        ['OWNER', 'MAINTAINER', 'MEMBER', 'GUEST'].map((role) => ({
          role,
          permissions:
            matrix.data.roles.find((item) => item.role === role)?.permissions ??
            [],
        })),
      )
  }, [matrix.data])
  const save = useMutation({
    mutationFn: () => updateEnvironmentPermissionMatrix(environmentId, roles),
    onSuccess: () => cache.invalidateQueries(),
  })
  const toggle = (role: string, point: string, enabled: boolean) =>
    setRoles((current) =>
      current.map((item) =>
        item.role === role
          ? {
              ...item,
              permissions: enabled
                ? [...item.permissions, point]
                : item.permissions.filter((value) => value !== point),
            }
          : item,
      ),
    )
  const error = matrix.error ?? save.error
  return (
    <section className="catalog-page">
      <Link to={`/projects/${projectId}/environments`}>{t('返回项目环境')}</Link>
      <h1>
        {t('项目 {{projectId}} / 环境 {{environmentId}} 权限', { projectId, environmentId })}
      </h1>
      {error ? (
        <Alert
          type="error"
          message={getApiErrorMessage(error, t('权限操作失败'))}
        />
      ) : null}
      <p>{t('Owner 始终拥有所有权限。更改仅影响当前环境。')}</p>
      <Table<PermissionRole>
        rowKey="role"
        loading={matrix.isLoading}
        dataSource={roles}
        pagination={false}
        columns={[
          {
            title: t('角色'),
            dataIndex: 'role',
            render: (role: string) => projectRoleLabel(role),
          },
          ...points.map((point) => ({
            title: point.name,
            render: (_: unknown, item: PermissionRole) => (
              <Checkbox
                aria-label={`${projectRoleLabel(item.role)} ${point.name}`}
                checked={
                  item.role === 'OWNER' || item.permissions.includes(point.key)
                }
                disabled={item.role === 'OWNER' || save.isPending}
                onChange={(event) =>
                  toggle(item.role, point.key, event.target.checked)
                }
              />
            ),
          })),
        ]}
      />
      <Space>
        <Button
          type="primary"
          loading={save.isPending}
          disabled={!matrix.data}
          onClick={() => save.mutate()}
        >
          {t('保存权限')}
        </Button>
        {save.isSuccess ? <span>{t('权限已保存')}</span> : null}
      </Space>
    </section>
  )
}
