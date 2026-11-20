import type { Rule } from 'antd/es/form'
import { t } from '@/i18n'

export const PASSWORD_POLICY_HINT = '8–32 个 ASCII 可见字符，不含空格；字母、数字、符号至少包含两类'

/**
 * Validates new passwords consistently across all account forms.
 * Parameters: value is the exact input, without trimming or normalization.
 * Returns: the violated rule, or null for a valid password.
 * Throws: none.
 */
export function passwordPolicyError(value: string): string | null {
  if (value.length < 8 || value.length > 32) return t('密码长度须为 8 至 32 个字符')
  if (!/^[\x21-\x7e]+$/.test(value)) return t('密码只能包含 ASCII 可见字符，不能包含空格或中文')
  const categories = [/[A-Za-z]/, /[0-9]/, /[^A-Za-z0-9]/]
  if (categories.filter((pattern) => pattern.test(value)).length < 2) {
    return t('密码须包含字母、数字、符号中的至少两类')
  }
  return null
}

/**
 * Builds form rules at render time so validation uses the selected language.
 * Parameters: none.
 * Returns: required and password-policy rules.
 * Throws: none; invalid input is reported through the rule validator.
 */
export function newPasswordRules(): Rule[] {
  return [
    {
      required: true,
      message: t('请输入密码'),
    },
    {
      validator: (_rule: unknown, value?: string) => {
        if (!value) return Promise.resolve()
        const error = passwordPolicyError(value)
        return error ? Promise.reject(new Error(error)) : Promise.resolve()
      },
    },
  ]
}
