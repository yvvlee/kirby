import { describe, expect, it } from 'vitest'

import { passwordPolicyError } from './password-policy'

describe('new password policy', () => {
  it.each(['abcd1234', 'ABCDEFG!', '1234567!', 'Abc123!x', 'a'.repeat(31) + '1'])('accepts valid combinations: %s', (value) => {
    expect(passwordPolicyError(value)).toBeNull()
  })

  it.each(Array.from({ length: 94 }, (_, index) => String.fromCharCode(33 + index)))('accepts visible ASCII character %s', (character) => {
    expect(passwordPolicyError('abcd123' + character)).toBeNull()
  })

  it.each(['', 'Abc123!', 'a'.repeat(32) + '1', 'abcdefgh', 'ABCDefgh', '12345678', '!@#$%^&*', 'abcd123 ', 'abcd123\t', 'abcd123\n', 'abcd123\x00', 'abcd123\x7f', 'abcd123é', 'abcd123中', 'abcd123😀'])('rejects invalid input: %s', (value) => {
    expect(passwordPolicyError(value)).not.toBeNull()
  })
})
