import client from './client'

function environmentPath(environmentId, suffix = '') {
  const id = String(environmentId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('environmentId must be a positive integer')
  }
  return `/admin/environments/${id}${suffix}`
}

export async function listEnvironments() {
  const { data } = await client.get('/admin/environments')
  return data
}

export async function getMyPermissions(environmentId) {
  const { data } = await client.get(
    environmentPath(environmentId, '/my-permissions'),
  )
  return data
}

export async function createEnvironment(environment) {
  const { data } = await client.post('/admin/environments', environment)
  return data
}

export async function updateEnvironment(environmentId, environment) {
  const { data } = await client.put(
    environmentPath(environmentId),
    {
      ...environment,
      environment_id: environmentId,
    },
  )
  return data
}
