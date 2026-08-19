import { Empty, Typography } from 'antd'
import { useParams } from 'react-router-dom'

import { positiveRouteId } from '@/app/routing'

type Props = {
  title: string
  validateProject?: boolean
  validateConfig?: boolean
}

export default function ResourcePlaceholder({ title, validateProject, validateConfig }: Props) {
  const params = useParams()
  if (validateProject) positiveRouteId('projectId', params.projectId)
  if (validateConfig) positiveRouteId('configId', params.configId)
  return (
    <section className="content-section" aria-labelledby="resource-title">
      <Typography.Title id="resource-title" level={2}>{title}</Typography.Title>
      <Empty description="暂无数据" />
    </section>
  )
}
