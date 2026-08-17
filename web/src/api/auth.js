import client from './client'

export async function login(credentials) {
  const { data } = await client.post('/auth/login', credentials, {
    skipAuthRefresh: true,
    skipAccessToken: true,
  })
  return data
}

export async function refreshSession() {
  const { data } = await client.post('/auth/refresh', null, {
    skipAuthRefresh: true,
    skipAccessToken: true,
  })
  return data
}

export async function logout() {
  await client.post('/auth/logout', null, {
    skipAuthRefresh: true,
  })
}

export async function getCurrentUser() {
  const { data } = await client.get('/auth/me')
  return data
}
