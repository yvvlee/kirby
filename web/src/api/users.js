import client from './client'

function userPath(userId, suffix = '') {
  const id = String(userId)
  if (!/^[1-9]\d*$/.test(id)) {
    throw new TypeError('userId must be a positive integer')
  }
  return `/admin/users/${id}${suffix}`
}

export async function listUsers() {
  const { data } = await client.get('/admin/users')
  return data
}

export async function createUser(user) {
  const { data } = await client.post('/admin/users', user)
  return data
}

export async function updateUser(userId, user) {
  const { data } = await client.put(userPath(userId), {
    ...user,
    user_id: userId,
  })
  return data
}

export async function updateUserPassword(userId, password) {
  const { data } = await client.put(userPath(userId, '/password'), {
    user_id: userId,
    password,
  })
  return data
}

export async function updateUserStatus(userId, enabled, version) {
  const { data } = await client.put(userPath(userId, '/status'), {
    user_id: userId,
    enabled,
    version,
  })
  return data
}
