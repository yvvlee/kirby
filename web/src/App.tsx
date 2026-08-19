import { Suspense } from 'react'
import { Spin } from 'antd'

import { ApplicationRouter } from '@/app/router'

export default function App() {
  return (
    <Suspense fallback={<div className="route-loading"><Spin size="large" /></div>}>
      <ApplicationRouter />
    </Suspense>
  )
}
