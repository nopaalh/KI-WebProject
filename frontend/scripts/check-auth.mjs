import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'

const origin = 'http://localhost:5173'
const username = `auth_check_${randomUUID().replaceAll('-', '')}`
const password = randomUUID()
const account = { name: 'Synthetic Auth Check', username, password }
const post = (path, body) => fetch(`${origin}/api/users/${path}`, {
  method: 'POST',
  signal: AbortSignal.timeout(15000),
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})
const me = (cookie) => fetch(`${origin}/api/users/me`, {
  signal: AbortSignal.timeout(15000),
  headers: cookie ? { Cookie: cookie } : {},
})

assert.equal((await me()).status, 401)
assert.equal((await post('register', { ...account, role: 'doctor' })).status, 201)
assert.equal((await post('register', account)).status, 409)
assert.equal((await post('login', { username, password: 'incorrect' })).status, 401)

const login = await post('login', { username, password })
assert.equal(login.status, 200)
const setCookie = login.headers.get('set-cookie')
assert.ok(setCookie?.includes('access_token='))
assert.match(setCookie, /;\s*HttpOnly/i)
assert.match(setCookie, /;\s*SameSite=Lax/i)
const response = await me(setCookie.split(';')[0])
assert.equal(response.status, 200)
const profile = await response.json()
assert.deepEqual(Object.keys(profile).sort(), ['id', 'name', 'role', 'username'])
assert.ok(Number.isInteger(profile.id) && profile.id > 0)
assert.equal(profile.name, account.name)
assert.equal(profile.username, username)
assert.equal(profile.role, 'patient')
assert.equal((await me('access_token=invalid')).status, 401)
console.log(`PASS: registration, duplicate username, credentials, cookie, and /me through the proxy. Created synthetic account: ${username}`)
