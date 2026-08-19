import { describe, expect, it } from 'vitest'

import { positiveRouteId, safeRedirect } from './routing'
import { queryKeys } from './query-keys'

describe('routing boundaries', () => {
  it('accepts only local absolute redirects', () => {
    expect(safeRedirect('/projects?tab=all')).toBe('/projects?tab=all')
    expect(safeRedirect('//example.com/session')).toBe('/')
    expect(safeRedirect('https://example.com')).toBe('/')
    expect(safeRedirect(null)).toBe('/')
  })

  it('requires positive route identifiers', () => {
    expect(positiveRouteId('projectId', '12')).toBe(12)
    expect(() => positiveRouteId('projectId', '../admin')).toThrow(
      'projectId must be a positive integer',
    )
    expect(() => positiveRouteId('projectId', '0')).toThrow(
      'projectId must be a positive integer',
    )
  })

  it('includes the environment in every scoped query key', () => {
    expect(queryKeys.projects(11)[1]).toBe('11')
    expect(queryKeys.configs(12, 7)).toEqual([
      'environment', '12', 'project', '7', 'configs',
    ])
    expect(queryKeys.models(13, 7, 31)).toEqual([
      'environment', '13', 'project', '7', 'config', '31', 'models',
    ])
    expect(queryKeys.enums(14, 7, 31)).toEqual([
      'environment', '14', 'project', '7', 'config', '31', 'enums',
    ])
    expect(queryKeys.snapshots(15, 7, 31)).toEqual([
      'environment', '15', 'project', '7', 'config', '31', 'snapshots',
    ])
    expect(queryKeys.apiKeys(16, 2)[1]).toBe('16')
  })

  it('uses an unfiltered list key as the prefix of filtered requests', () => {
    expect(queryKeys.projects(11, { page: 2 }).slice(0, -1)).toEqual(queryKeys.projects(11))
    expect(queryKeys.configs(11, 7, { keyword: 'api' }).slice(0, -1)).toEqual(queryKeys.configs(11, 7))
    expect(queryKeys.snapshots(11, 7, 31, { page: 2 }).slice(0, -1)).toEqual(queryKeys.snapshots(11, 7, 31))
  })
})
