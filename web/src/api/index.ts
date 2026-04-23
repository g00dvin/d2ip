import axios from 'axios'

const api = axios.create({
  baseURL: '/',
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.response.use(
  (r) => r,
  (e) => {
    if (e.response?.data?.error) {
      return Promise.reject(new Error(e.response.data.error))
    }
    return Promise.reject(e)
  },
)

export default api

export interface PipelineStatus {
  running: boolean
  run_id: string
  started: string
  report: {
    run_id: string
    duration: number
    domains: number
    resolved: number
    failed: number
    ipv4_out: number
    ipv6_out: number
  } | null
}

export interface PipelineHistory {
  history: Array<{
    run_id: string
    duration: number
    domains: number
    resolved: number
    failed: number
    ipv4_out: number
    ipv6_out: number
  }>
}

export interface RoutingSnapshot {
  backend: string
  applied_at: string
  v4: string[]
  v6: string[]
}

export interface DryRunResult {
  v4_plan: { add: string[]; remove: string[] }
  v6_plan: { add: string[]; remove: string[] }
  v4_diff: string
  v6_diff: string
  message?: string
}

export interface SettingsResponse {
  config: Record<string, unknown>
  defaults: Record<string, unknown>
  overrides: Record<string, string>
}

export interface CategoryInfo {
  code: string
  attrs?: string[]
  domain_count: number
}

export interface CategoriesResponse {
  configured: CategoryInfo[]
  available: string[]
}

export interface CategoryDomainsResponse {
  code: string
  domains: string[]
  page: number
  per_page: number
  total: number
  has_more: boolean
}

export interface CacheStats {
  domains: number
  records_total: number
  records_v4: number
  records_v6: number
  records_valid: number
  records_failed: number
  oldest_updated: string
  newest_updated: string
}

export interface SourceInfo {
  available: boolean
  fetched_at?: string
  size?: number
  etag?: string
  sha256?: string
  last_modified?: string
}