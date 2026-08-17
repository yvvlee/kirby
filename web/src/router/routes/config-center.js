const ConfigDetailPage = () =>
  import('@/features/config-center/configs/ConfigDetailPage.vue')
const ConfigsPage = () =>
  import('@/features/config-center/configs/ConfigsPage.vue')
const ProjectsPage = () =>
  import('@/features/config-center/projects/ProjectsPage.vue')

function positiveRouteID(name, value) {
  const id = Number(value)
  if (!Number.isSafeInteger(id) || id <= 0) {
    throw new TypeError(`${name} must be a positive integer`)
  }
  return id
}

const configCenterRoutes = [
  {
    path: 'projects',
    name: 'project-list',
    component: ProjectsPage,
    meta: { permission: 'project:read' },
  },
  {
    path: 'projects/:projectId/configs',
    name: 'project-configs',
    component: ConfigsPage,
    props: (route) => ({
      projectId: positiveRouteID('projectId', route.params.projectId),
    }),
    meta: { permission: 'config:read' },
  },
  {
    path: 'projects/:projectId/configs/:configId',
    name: 'config-detail',
    component: ConfigDetailPage,
    props: (route) => ({
      projectId: positiveRouteID('projectId', route.params.projectId),
      configId: positiveRouteID('configId', route.params.configId),
    }),
    meta: { permission: 'config:read' },
  },
]

export default configCenterRoutes
