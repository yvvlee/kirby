import { lazy } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'

import { AuthGate, PermissionGate } from './guards'
import { SYSTEM_PERMISSIONS } from '@/domain/access'

const LoginPage = lazy(() => import('@/routes/LoginPage'))
const InvitationPage = lazy(() => import('@/routes/InvitationPage'))
const RegisterPage = lazy(() => import('@/routes/RegisterPage'))
const SetupPage = lazy(() => import('@/routes/SetupPage'))
const ForbiddenPage = lazy(() => import('@/routes/ForbiddenPage'))
const NotFoundPage = lazy(() => import('@/routes/NotFoundPage'))
const AppLayout = lazy(() => import('@/layout/AppLayout'))
const HomePage = lazy(() => import('@/routes/HomePage'))
const SystemLayout = lazy(() => import('@/layout/SystemLayout'))
const SystemHomePage = lazy(() => import('@/routes/SystemHomePage'))
const EnvironmentsPage = lazy(() => import('@/features/system/EnvironmentsPage'))
const UsersPage = lazy(() => import('@/features/system/UsersPage'))
const SystemSettingsPage = lazy(() => import('@/features/system/SystemSettingsPage'))
const ProjectsPage = lazy(() => import('@/features/config-center/projects/ProjectsPage'))
const ConfigsPage = lazy(() => import('@/features/config-center/configs/ConfigsPage'))
const ConfigDetailPage = lazy(() => import('@/features/config-center/configs/ConfigDetailPage'))
const ProjectMembersPage = lazy(() => import('@/features/config-center/projects/ProjectMembersPage'))
const EnvironmentPermissionsPage = lazy(() => import('@/features/system/EnvironmentPermissionsPage'))

const protect = (permission: string, element: React.ReactNode) => (
  <PermissionGate permission={permission}>{element}</PermissionGate>
)

export function ApplicationRouter() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/invitations/:token" element={<InvitationPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/setup" element={<SetupPage />} />
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
          element={<ProjectsPage />}
        />
        <Route
          path="projects/:projectId/configs"
          element={<ConfigsPage />}
        />
        <Route
          path="projects/:projectId/configs/:configId"
          element={protect('config:read', <ConfigDetailPage />)}
        />
        <Route path="projects/:projectId/environments" element={<EnvironmentsPage />} />
        <Route path="projects/:projectId/members" element={<ProjectMembersPage />} />
        <Route path="projects/:projectId/environments/:environmentId/permissions" element={<EnvironmentPermissionsPage />} />
        <Route path="system" element={<SystemLayout />}>
          <Route index element={<SystemHomePage />} />
          <Route path="users" element={protect(SYSTEM_PERMISSIONS.manageUsers, <UsersPage />)} />
          <Route path="settings" element={<SystemSettingsPage />} />
        </Route>
        <Route path="403" element={<ForbiddenPage />} />
        <Route path="404" element={<NotFoundPage />} />
        <Route path="*" element={<Navigate to="/404" replace />} />
      </Route>
    </Routes>
  )
}
