import { client } from './client'
import type {
  CategoriesResponse, CategoryDomainsResponse,
  PipelineStatus, PipelineHistory, RoutingSnapshot,
  DryRunResult, SettingsResponse, CacheStats, SourceInfo,
  PolicyConfig,
} from './types'

// Pipeline
export async function getPipelineStatus(): Promise<PipelineStatus> {
  const { data } = await client.get<PipelineStatus>('/pipeline/status')
  return data
}

export async function getPipelineHistory(): Promise<PipelineHistory> {
  const { data } = await client.get<PipelineHistory>('/api/pipeline/history')
  return data
}

export async function runPipeline(body?: { force_resolve?: boolean; dry_run?: boolean; skip_routing?: boolean }): Promise<void> {
  await client.post('/pipeline/run', body ?? {})
}

export async function cancelPipeline(): Promise<void> {
  await client.post('/pipeline/cancel')
}

// Categories
export async function getCategories(): Promise<CategoriesResponse> {
  const { data } = await client.get<CategoriesResponse>('/api/categories')
  return data
}

export async function addCategory(code: string): Promise<void> {
  await client.post('/api/categories', { code })
}

export async function removeCategory(code: string): Promise<void> {
  await client.delete(`/api/categories/${code}`)
}

export async function getCategoryDomains(code: string, params?: { page?: number; per_page?: number }): Promise<CategoryDomainsResponse> {
  const { data } = await client.get<CategoryDomainsResponse>(`/api/categories/${code}/domains`, { params })
  return data
}

// Cache
export async function getCacheStats(): Promise<CacheStats> {
  const { data } = await client.get<CacheStats>('/api/cache/stats')
  return data
}

export async function vacuumCache(): Promise<{ deleted: number }> {
  const { data } = await client.post('/api/cache/vacuum')
  return data
}

// Source
export async function getSourceInfo(): Promise<SourceInfo> {
  const { data } = await client.get<SourceInfo>('/api/source/info')
  return data
}

export async function fetchSource(): Promise<{ status: string; fetched_at: string; size: number; sha256: string }> {
  const { data } = await client.post('/api/source/fetch')
  return data
}

// Settings
export async function getSettings(): Promise<SettingsResponse> {
  const { data } = await client.get<SettingsResponse>('/api/settings')
  return data
}

export async function updateSettings(overrides: Record<string, string>): Promise<void> {
  await client.put('/api/settings', overrides)
}

export async function deleteSetting(key: string): Promise<void> {
  await client.delete(`/api/settings/${key}`)
}

// Policies
export async function getPolicies(): Promise<{ policies: PolicyConfig[] }> {
  const { data } = await client.get('/api/policies')
  return data
}

export async function getPolicy(name: string): Promise<PolicyConfig> {
  const { data } = await client.get(`/api/policies/${name}`)
  return data
}

export async function createPolicy(policy: PolicyConfig): Promise<void> {
  await client.post('/api/policies', policy)
}

export async function updatePolicy(name: string, policy: PolicyConfig): Promise<void> {
  await client.put(`/api/policies/${name}`, policy)
}

export async function deletePolicy(name: string): Promise<void> {
  await client.delete(`/api/policies/${name}`)
}

// Routing
export async function getRoutingSnapshot(): Promise<RoutingSnapshot> {
  const { data } = await client.get<RoutingSnapshot>('/routing/snapshot')
  return data
}

export async function dryRunRouting(body: { ipv4_prefixes: string[]; ipv6_prefixes: string[] }): Promise<DryRunResult> {
  const { data } = await client.post<DryRunResult>('/routing/dry-run', body)
  return data
}

export async function rollbackRouting(): Promise<void> {
  await client.post('/routing/rollback')
}

// Health
export async function getHealth(): Promise<{ status: string }> {
  const { data } = await client.get('/healthz')
  return data
}

// Version
export async function getVersion(): Promise<{ version: string; build_time: string }> {
  const { data } = await client.get('/api/version')
  return data
}

// Export
export async function downloadExport(policy: string, type: 'ipv4' | 'ipv6'): Promise<Blob> {
  const { data } = await client.get(`/api/export/download?policy=${encodeURIComponent(policy)}&type=${type}`, {
    responseType: 'blob',
  })
  return data
}

// Config export/import
export async function exportConfig(): Promise<Blob> {
  const { data } = await client.get('/api/config/export', {
    responseType: 'blob',
  })
  return data
}

export async function importConfig(overrides: Record<string, string>): Promise<void> {
  await client.post('/api/config/import', { overrides })
}
