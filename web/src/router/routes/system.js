import { SYSTEM_PERMISSIONS } from '@/features/system/access'

const EnvironmentsPage = () =>
  import('@/features/system/EnvironmentsPage.vue')
const MembersPage = () => import('@/features/system/MembersPage.vue')
const RolesPage = () => import('@/features/system/RolesPage.vue')
const SystemHome = () => import('@/features/system/SystemHome.vue')
const SystemLayout = () => import('@/features/system/SystemLayout.vue')
const UsersPage = () => import('@/features/system/UsersPage.vue')

const systemRoutes = [
  {
    path: 'system',
    component: SystemLayout,
    children: [
      {
        path: '',
        name: 'system-home',
        component: SystemHome,
      },
      {
        path: 'environments',
        name: 'system-environments',
        component: EnvironmentsPage,
        meta: { permission: SYSTEM_PERMISSIONS.manageEnvironments },
      },
      {
        path: 'users',
        name: 'system-users',
        component: UsersPage,
        meta: { permission: SYSTEM_PERMISSIONS.manageUsers },
      },
      {
        path: 'roles',
        name: 'system-roles',
        component: RolesPage,
        meta: { permission: SYSTEM_PERMISSIONS.manageRoles },
      },
      {
        path: 'members',
        name: 'environment-members',
        component: MembersPage,
        meta: { permission: SYSTEM_PERMISSIONS.manageMembers },
      },
    ],
  },
]

export default systemRoutes
