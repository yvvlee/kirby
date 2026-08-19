import { Typography } from 'antd'

import { useEnvironment } from '@/auth/environment-state'

export default function HomePage() {
  const { current } = useEnvironment()
  return (
    <section className="content-section" aria-labelledby="welcome-title">
      <p className="section-eyebrow">Kirby</p>
      <Typography.Title id="welcome-title" level={1}>配置管理平台</Typography.Title>
      <Typography.Text type="secondary">
        {current ? `当前环境：${current.name}` : '当前账号没有可用环境。'}
      </Typography.Text>
    </section>
  )
}
