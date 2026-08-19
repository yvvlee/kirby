export function safeRedirect(value: string | null | undefined): string {
  return value && value.startsWith('/') && !value.startsWith('//') ? value : '/'
}

export function positiveRouteId(name: string, value: string | undefined): number {
  const id = Number(value)
  if (!Number.isSafeInteger(id) || id <= 0) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}
