export function isForbidden(error) {
  return error?.response?.status === 403
}

export function actionErrorMessage(error, action) {
  if (isForbidden(error)) {
    return `没有权限${action}。当前页面已保留。`
  }
  return error?.response?.data?.message || error?.message || `${action}失败`
}

export function requireList(reply, name) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError(`${name} response does not contain list`)
  }
  return reply.list
}
