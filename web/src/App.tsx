import { Suspense } from 'react'
import { Spin } from 'antd'

import { ApplicationRouter } from '@/app/router'
import { SetupGate } from '@/app/setup-gate'

export default function App() {
  return (
    <Suspense fallback={<div className="route-loading"><Spin size="large" /></div>}>
      <SetupGate><ApplicationRouter /></SetupGate>
    </Suspense>
  )
}
