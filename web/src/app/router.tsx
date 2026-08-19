import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'

import { AuthGate, PermissionGate } from './guards'
import { SYSTEM_PERMISSIONS } from '@/domain/access'

const LoginPage = lazy(() => import('@/routes/LoginPage'))
const ForbiddenPage = lazy(() => import('@/routes/ForbiddenPage'))
const NotFoundPage = lazy(() => import('@/routes/NotFoundPage'))
const AppLayout = lazy(() => import('@/layout/AppLayout'))
const HomePage = lazy(() => import('@/routes/HomePage'))
const SystemLayout = lazy(() => import('@/layout/SystemLayout'))
const SystemHomePage = lazy(() => import('@/routes/SystemHomePage'))
const EnvironmentsPage = lazy(() => import('@/features/system/EnvironmentsPage'))
const UsersPage = lazy(() => import('@/features/system/UsersPage'))
const RolesPage = lazy(() => import('@/features/system/RolesPage'))
const MembersPage = lazy(() => import('@/features/system/MembersPage'))
const ProjectsPage = lazy(() => import('@/features/config-center/projects/ProjectsPage'))
const ConfigsPage = lazy(() => import('@/features/config-center/configs/ConfigsPage'))
const ConfigDetailPage = lazy(() => import('@/features/config-center/configs/ConfigDetailPage'))

const protect = (permission: string, element: React.ReactNode) => (
  <PermissionGate permission={permission}>{element}</PermissionGate>
)

export function ApplicationRouter() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <AuthGate>
            <AppLayout />
          </AuthGate>
        }
      >
        <Route index element={<HomePage />} />
        <Route
          path="projects"
          element={protect('project:read', <ProjectsPage />)}
        />
        <Route
          path="projects/:projectId/configs"
          element={protect('config:read', <ConfigsPage />)}
        />
        <Route
          path="projects/:projectId/configs/:configId"
          element={protect('config:read', <ConfigDetailPage />)}
        />
        <Route path="system" element={<SystemLayout />}>
          <Route index element={<SystemHomePage />} />
          <Route path="environments" element={protect(SYSTEM_PERMISSIONS.manageEnvironments, <EnvironmentsPage />)} />
          <Route path="users" element={protect(SYSTEM_PERMISSIONS.manageUsers, <UsersPage />)} />
          <Route path="roles" element={protect(SYSTEM_PERMISSIONS.manageRoles, <RolesPage />)} />
          <Route path="members" element={protect(SYSTEM_PERMISSIONS.manageMembers, <MembersPage />)} />
        </Route>
        <Route path="403" element={<ForbiddenPage />} />
        <Route path="404" element={<NotFoundPage />} />
        <Route path="*" element={<Navigate to="/404" replace />} />
      </Route>
    </Routes>
  )
}
