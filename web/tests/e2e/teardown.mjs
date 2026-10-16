import { spawnSync } from 'node:child_process'

export default function teardown(configuration) {
  const containerName = configuration.metadata.setupMode ? 'kirby-setup-mysql' : 'kirby-fe08-mysql'
  const result = spawnSync('docker', ['rm', '-f', containerName], {
    encoding: 'utf8',
  })
  if (result.status !== 0 && !result.stderr.includes('No such container')) {
    throw new Error(`failed to remove Kirby E2E MySQL container: ${result.stderr}`)
  }
}
