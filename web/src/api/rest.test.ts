import { describe, it, expect, vi, beforeEach } from 'vitest'
import { client } from './client'
import * as api from './rest'

vi.mock('./client', () => ({
  client: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('REST API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('getPipelineStatus', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { running: false, report: null } })
    const result = await api.getPipelineStatus()
    expect(client.get).toHaveBeenCalledWith('/pipeline/status')
    expect(result).toEqual({ running: false, report: null })
  })

  it('getPipelineHistory', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { history: [] } })
    const result = await api.getPipelineHistory()
    expect(client.get).toHaveBeenCalledWith('/api/pipeline/history')
    expect(result).toEqual({ history: [] })
  })

  it('runPipeline', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.runPipeline({ force_resolve: true })
    expect(client.post).toHaveBeenCalledWith('/pipeline/run', { force_resolve: true })
  })

  it('runPipeline without body', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.runPipeline()
    expect(client.post).toHaveBeenCalledWith('/pipeline/run', {})
  })

  it('cancelPipeline', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.cancelPipeline()
    expect(client.post).toHaveBeenCalledWith('/pipeline/cancel')
  })

  it('getCategories', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { configured: [], available: [] } })
    const result = await api.getCategories()
    expect(client.get).toHaveBeenCalledWith('/api/categories')
    expect(result).toEqual({ configured: [], available: [] })
  })

  it('addCategory', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.addCategory('geosite:ru')
    expect(client.post).toHaveBeenCalledWith('/api/categories', { code: 'geosite:ru' })
  })

  it('removeCategory', async () => {
    vi.mocked(client.delete).mockResolvedValue({})
    await api.removeCategory('geosite:ru')
    expect(client.delete).toHaveBeenCalledWith('/api/categories/geosite:ru')
  })

  it('getCategoryDomains', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { code: 'test', domains: [], page: 1, per_page: 100, total: 0, has_more: false } })
    const result = await api.getCategoryDomains('test', { page: 1, per_page: 10 })
    expect(client.get).toHaveBeenCalledWith('/api/categories/test/domains', { params: { page: 1, per_page: 10 } })
    expect(result.code).toBe('test')
  })

  it('getCacheStats', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { domains: 100 } })
    const result = await api.getCacheStats()
    expect(client.get).toHaveBeenCalledWith('/api/cache/stats')
    expect(result.domains).toBe(100)
  })

  it('vacuumCache', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: { deleted: 5 } })
    const result = await api.vacuumCache()
    expect(client.post).toHaveBeenCalledWith('/api/cache/vacuum')
    expect(result.deleted).toBe(5)
  })

  it('getSources', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { sources: [] } })
    const result = await api.getSources()
    expect(client.get).toHaveBeenCalledWith('/api/sources')
    expect(result).toEqual({ sources: [] })
  })

  it('getSource', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { id: 'test' } })
    const result = await api.getSource('test')
    expect(client.get).toHaveBeenCalledWith('/api/sources/test')
    expect(result).toEqual({ id: 'test' })
  })

  it('createSource', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.createSource({ id: 'test', provider: 'plaintext', prefix: 't', enabled: true })
    expect(client.post).toHaveBeenCalledWith('/api/sources', { id: 'test', provider: 'plaintext', prefix: 't', enabled: true })
  })

  it('updateSource', async () => {
    vi.mocked(client.put).mockResolvedValue({})
    await api.updateSource('test', { id: 'test', provider: 'plaintext', prefix: 't', enabled: true })
    expect(client.put).toHaveBeenCalledWith('/api/sources/test', { id: 'test', provider: 'plaintext', prefix: 't', enabled: true })
  })

  it('deleteSource', async () => {
    vi.mocked(client.delete).mockResolvedValue({})
    await api.deleteSource('test')
    expect(client.delete).toHaveBeenCalledWith('/api/sources/test')
  })

  it('refreshSource', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: { status: 'ok', info: { id: 'test' } } })
    const result = await api.refreshSource('test')
    expect(client.post).toHaveBeenCalledWith('/api/sources/test/refresh')
    expect(result.status).toBe('ok')
  })

  it('uploadSourceFile', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: { path: '/tmp/test.txt' } })
    const file = new File(['content'], 'test.txt')
    const result = await api.uploadSourceFile(file)
    expect(client.post).toHaveBeenCalledWith('/api/sources/upload', expect.any(FormData), {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    expect(result.path).toBe('/tmp/test.txt')
  })

  it('getSettings', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { config: {} } })
    const result = await api.getSettings()
    expect(client.get).toHaveBeenCalledWith('/api/settings')
    expect(result).toEqual({ config: {} })
  })

  it('updateSettings', async () => {
    vi.mocked(client.put).mockResolvedValue({})
    await api.updateSettings({ listen: ':9090' })
    expect(client.put).toHaveBeenCalledWith('/api/settings', { listen: ':9090' })
  })

  it('deleteSetting', async () => {
    vi.mocked(client.delete).mockResolvedValue({})
    await api.deleteSetting('listen')
    expect(client.delete).toHaveBeenCalledWith('/api/settings/listen')
  })

  it('getPolicies', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { policies: [] } })
    const result = await api.getPolicies()
    expect(client.get).toHaveBeenCalledWith('/api/policies')
    expect(result).toEqual({ policies: [] })
  })

  it('getPolicy', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { name: 'test' } })
    const result = await api.getPolicy('test')
    expect(client.get).toHaveBeenCalledWith('/api/policies/test')
    expect(result).toEqual({ name: 'test' })
  })

  it('createPolicy', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.createPolicy({ name: 'test', enabled: true, categories: [], backend: 'none', dry_run: false, export_format: 'plain' })
    expect(client.post).toHaveBeenCalledWith('/api/policies', { name: 'test', enabled: true, categories: [], backend: 'none', dry_run: false, export_format: 'plain' })
  })

  it('updatePolicy', async () => {
    vi.mocked(client.put).mockResolvedValue({})
    await api.updatePolicy('test', { name: 'test', enabled: true, categories: [], backend: 'none', dry_run: false, export_format: 'plain' })
    expect(client.put).toHaveBeenCalledWith('/api/policies/test', { name: 'test', enabled: true, categories: [], backend: 'none', dry_run: false, export_format: 'plain' })
  })

  it('deletePolicy', async () => {
    vi.mocked(client.delete).mockResolvedValue({})
    await api.deletePolicy('test')
    expect(client.delete).toHaveBeenCalledWith('/api/policies/test')
  })

  it('getRoutingSnapshot', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { backend: 'nftables' } })
    const result = await api.getRoutingSnapshot()
    expect(client.get).toHaveBeenCalledWith('/routing/snapshot')
    expect(result).toEqual({ backend: 'nftables' })
  })

  it('dryRunRouting', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: { v4_plan: { add: [], remove: [] }, v6_plan: { add: [], remove: [] }, v4_diff: '', v6_diff: '' } })
    const result = await api.dryRunRouting({ ipv4_prefixes: ['1.2.3.0/24'], ipv6_prefixes: [] })
    expect(client.post).toHaveBeenCalledWith('/routing/dry-run', { ipv4_prefixes: ['1.2.3.0/24'], ipv6_prefixes: [] })
    expect(result.v4_diff).toBe('')
  })

  it('rollbackRouting', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.rollbackRouting()
    expect(client.post).toHaveBeenCalledWith('/routing/rollback')
  })

  it('getHealth', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { status: 'ok' } })
    const result = await api.getHealth()
    expect(client.get).toHaveBeenCalledWith('/healthz')
    expect(result).toEqual({ status: 'ok' })
  })

  it('getVersion', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { version: '1.0.0', build_time: '2024-01-01' } })
    const result = await api.getVersion()
    expect(client.get).toHaveBeenCalledWith('/api/version')
    expect(result.version).toBe('1.0.0')
  })

  it('downloadExport', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: new Blob(['1.2.3.0/24']) })
    const result = await api.downloadExport('test', 'ipv4')
    expect(client.get).toHaveBeenCalledWith('/api/export/download?policy=test&type=ipv4', { responseType: 'blob' })
    expect(result).toBeInstanceOf(Blob)
  })

  it('exportConfig', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: new Blob(['{}']) })
    const result = await api.exportConfig()
    expect(client.get).toHaveBeenCalledWith('/api/config/export', { responseType: 'blob' })
    expect(result).toBeInstanceOf(Blob)
  })

  it('importConfig', async () => {
    vi.mocked(client.post).mockResolvedValue({})
    await api.importConfig({ listen: ':9090' })
    expect(client.post).toHaveBeenCalledWith('/api/config/import', { overrides: { listen: ':9090' } })
  })
})
