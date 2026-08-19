import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'

export default function ForbiddenPage() {
  return <Result status="403" title="403" subTitle="当前账号在所选环境中没有访问权限。" extra={<Link to="/"><Button type="primary">返回首页</Button></Link>} />
}
