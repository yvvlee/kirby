import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'
import { useTranslation } from '@/i18n'

export default function ForbiddenPage() {
  const { t } = useTranslation()
  return <Result status="403" title="403" subTitle={t('当前账号在所选环境中没有访问权限。')} extra={<Link to="/"><Button type="primary">{t('返回首页')}</Button></Link>} />
}
