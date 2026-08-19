import { spawn, spawnSync } from 'node:child_process'
import { existsSync, readFileSync, writeFileSync, chmodSync, rmSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { tmpdir } from 'node:os'

const webRoot = resolve(import.meta.dirname, '../..')
const repositoryRoot = resolve(webRoot, '..')
const serverRoot = join(repositoryRoot, 'server')
const publicRoot = join(webRoot, 'dist')
const configPath = join(import.meta.dirname, 'fixtures/config.e2e.yaml')
const containerName = 'kirby-fe08-mysql'
const mysqlCredential = 'kirby-e2e-password'
const binaryPath = join(tmpdir(), `kirby-e2e-${process.pid}`)
const passwordPath = join(tmpdir(), `kirby-e2e-password-${process.pid}`)
const objectPath = '/tmp/kirby-e2e-objects'
const children = new Set()
let stopping = false

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { encoding: 'utf8', ...options })
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed\n${result.stdout || ''}${result.stderr || ''}`)
  }
  return result
}

function ignore(command, args) {
  spawnSync(command, args, { stdio: 'ignore' })
}

async function waitFor(check, description, timeout = 180_000) {
  const deadline = Date.now() + timeout
  let lastError
  while (Date.now() < deadline) {
    try {
      if (await check()) return
    } catch (error) {
      lastError = error
    }
    await new Promise((resolveWait) => setTimeout(resolveWait, 500))
  }
  throw new Error(`timed out waiting for ${description}: ${lastError?.message || 'not ready'}`)
}

async function cleanup(exitCode = 0) {
  if (stopping) return
  stopping = true
  for (const child of children) child.kill('SIGTERM')
  ignore('docker', ['rm', '-f', containerName])
  for (const path of [binaryPath, passwordPath, objectPath]) {
    if (existsSync(path)) rmSync(path, { recursive: true, force: true })
  }
  process.exit(exitCode)
}

process.on('SIGINT', () => cleanup(130))
process.on('SIGTERM', () => cleanup(0))
process.on('uncaughtException', (error) => {
  console.error(error)
  cleanup(1)
})

ignore('docker', ['rm', '-f', containerName])
const mysql = spawn('docker', [
  'run', '--rm', '--name', containerName,
  '-p', '127.0.0.1:33306:3306',
  '-e', `MYSQL_ROOT_PASSWORD=${mysqlCredential}`,
  '-e', 'MYSQL_DATABASE=kirby',
  process.env.KIRBY_E2E_MYSQL_IMAGE || 'mysql@sha256:c11782aa2a96624c1efc121768641d96954faa136d6aa82751b032d8c426ffbc',
], { stdio: ['ignore', 'inherit', 'inherit'] })
children.add(mysql)
mysql.once('exit', (code) => { if (!stopping) cleanup(code || 1) })

await waitFor(() => spawnSync('docker', [
  'exec', containerName, 'mysql', '-h127.0.0.1', '-P3306',
  '-uroot', `-p${mysqlCredential}`, '-e', 'SELECT 1',
], { stdio: 'ignore' }).status === 0, 'MySQL')

const schema = readFileSync(join(repositoryRoot, 'deploy/schema.sql'))
const schemaResult = spawnSync('docker', [
  'exec', '-i', containerName, 'mysql', '-uroot', `-p${mysqlCredential}`, 'kirby',
], { input: schema, encoding: 'utf8' })
if (schemaResult.status !== 0) throw new Error(schemaResult.stderr)

run('go', ['build', '-o', binaryPath, '.'], { cwd: serverRoot })
writeFileSync(passwordPath, 'kirby-e2e-admin-password\n', { mode: 0o600 })
chmodSync(passwordPath, 0o600)
run(binaryPath, [
  'create-admin', '--config', configPath, '--username', 'admin',
  '--display-name', 'E2E Admin', '--password-file', passwordPath,
], { cwd: serverRoot })

const backend = spawn(binaryPath, ['serve', '--config', configPath, '--web-root', publicRoot], {
  cwd: serverRoot,
  stdio: ['ignore', 'inherit', 'inherit'],
})
children.add(backend)
backend.once('exit', (code) => { if (!stopping) cleanup(code || 1) })
await waitFor(async () => (await fetch('http://127.0.0.1:14173/login')).ok, 'Kirby web application')
console.log('Kirby E2E ready at http://127.0.0.1:14173')
await new Promise(() => {})
