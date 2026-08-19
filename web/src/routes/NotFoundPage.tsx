import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'

export default function NotFoundPage() {
  return <Result status="404" title="404" subTitle="请求的页面不存在或已经被移除。" extra={<Link to="/"><Button type="primary">返回首页</Button></Link>} />
}
