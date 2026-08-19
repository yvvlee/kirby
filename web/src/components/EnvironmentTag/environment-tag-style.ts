import type { CSSProperties } from 'react'

import type { Environment } from '@/api/types'

function hash(value: string): number {
  let result = 0
  for (const character of value) {
    result = (result * 31 + (character.codePointAt(0) ?? 0)) % 360
  }
  return result
}

export function environmentTagStyle(
  environment: Environment | null,
): CSSProperties {
  const label = environment?.name ?? '未选择环境'
  const hue = hash(environment?.key ?? label)
  return {
    backgroundColor: `hsl(${hue} 70% 92%)`,
    borderColor: `hsl(${hue} 55% 72%)`,
    color: `hsl(${hue} 55% 28%)`,
  }
}
