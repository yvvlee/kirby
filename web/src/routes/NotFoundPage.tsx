import { Button, Result } from 'antd'
import { Link } from 'react-router-dom'
import { useTranslation } from '@/i18n'

export default function NotFoundPage() {
  const { t } = useTranslation()
  return <Result status="404" title="404" subTitle={t('请求的页面不存在或已经被移除。')} extra={<Link to="/"><Button type="primary">{t('返回首页')}</Button></Link>} />
}
