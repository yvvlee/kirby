import { Select } from 'antd'
import { setLocale, useTranslation } from './index'

/**
 * Exposes the same language choice on public and authenticated pages.
 * Parameters: none.
 * Returns: the language selector.
 * Throws: none.
 */
export default function LanguageSwitcher() {
  const { t, locale } = useTranslation()
  return <Select
    data-testid="language-selector"
    aria-label={t('语言')}
    value={locale}
    onChange={setLocale}
    popupMatchSelectWidth={false}
    options={[
      {
        value: 'zh-CN',
        label: '简体中文',
      },
      {
        value: 'en-US',
        label: 'English',
      },
    ]}
  />
}
