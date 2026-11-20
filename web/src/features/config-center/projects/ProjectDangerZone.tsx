import { Alert, Button, Input, Modal } from 'antd'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { deleteProject } from '@/api/projects'
import { getApiErrorMessage } from '@/api/errors'
import { useTranslation } from '@/i18n'
import type { Identifier } from '@/api/types'

type Props = {
  projectId: Identifier
  projectName: string
}

/**
 * Requires an exact project-name confirmation to prevent accidental deletion.
 * Parameters: props identifies the project and its saved name.
 * Returns: the danger zone and confirmation dialog.
 * Throws: none; request failures appear in the dialog.
 */
export default function ProjectDangerZone({ projectId, projectName }: Props) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [confirmation, setConfirmation] = useState('')
  const cache = useQueryClient()
  const navigate = useNavigate()
  const remove = useMutation({
    mutationFn: () => deleteProject(projectId),
    onSuccess: () => {
      cache.clear()
      navigate('/projects', { replace: true })
    },
  })
  const canDelete = confirmation === projectName
  const openConfirmation = () => {
    setConfirmation('')
    remove.reset()
    setOpen(true)
  }
  return <section className="project-danger-zone" aria-label={t('危险区')}>
    <h3>{t('危险区')}</h3>
    <p>{t('删除项目会永久删除其环境、配置、快照、成员及 API Key，无法恢复。')}</p>
    <Button danger onClick={openConfirmation}>{t('删除项目')}</Button>
    <Modal
      title={t('永久删除项目')}
      open={open}
      okText={t('永久删除')}
      okButtonProps={{ danger: true, disabled: !canDelete }}
      confirmLoading={remove.isPending}
      cancelButtonProps={{ disabled: remove.isPending }}
      closable={!remove.isPending}
      maskClosable={false}
      onCancel={() => setOpen(false)}
      onOk={() => { if (canDelete) remove.mutate() }}
      destroyOnHidden
    >
      <p>{t('请输入项目名称 {{projectName}} 以确认删除。', { projectName })}</p>
      <Input aria-label={t('确认项目名称')} value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" disabled={remove.isPending} />
      {remove.error ? <Alert type="error" message={getApiErrorMessage(remove.error, t('删除项目失败'))} /> : null}
    </Modal>
  </section>
}
