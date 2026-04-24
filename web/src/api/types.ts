export interface PipelineStatus {
  running: boolean
  run_id: number
  started: string
  report: {
    run_id: number
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
    run_id: number
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
  v4: string[] | number
  v6: string[] | number
  policies?: number
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
  domains_valid: number
  domains_failed: number
  domains_nxdomain: number
  records_total: number
  records_v4: number
  records_v6: number
  records_valid: number
  records_failed: number
  records_nxdomain: number
  oldest_updated: number
  newest_updated: number
}

export interface SourceInfo {
  available: boolean
  fetched_at?: string
  size?: number
  etag?: string
  sha256?: string
  last_modified?: string
}

export interface PolicyConfig {
  name: string
  enabled: boolean
  categories: string[]
  backend: 'iproute2' | 'nftables' | 'none'
  table_id?: number
  iface?: string
  nft_table?: string
  nft_set_v4?: string
  nft_set_v6?: string
  dry_run: boolean
  export_format: string
}

export interface PolicyReport {
  name: string
  domains: number
  resolved: number
  failed: number
  ipv4_out: number
  ipv6_out: number
  duration_ms: number
}
