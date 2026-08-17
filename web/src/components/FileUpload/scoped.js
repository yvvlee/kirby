import FileUpload from './FileUpload.vue'

function positiveId(name, value) {
  const id = String(value)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

export function createScopedFileUpload(environmentId, projectId) {
  const environment = Number(positiveId('environmentId', environmentId))
  const project = Number(positiveId('projectId', projectId))

  return {
    name: `ScopedFileUpload_${environment}_${project}`,
    functional: true,
    render(createElement, context) {
      return createElement(
        FileUpload,
        {
          ...context.data,
          attrs: { ...(context.data.attrs || {}) },
          props: {
            ...(context.props || {}),
            ...(context.data.props || {}),
            environmentId: environment,
            projectId: project,
          },
          on: {
            ...(context.data.on || {}),
            ...(context.listeners || {}),
          },
        },
        context.children,
      )
    },
  }
}
