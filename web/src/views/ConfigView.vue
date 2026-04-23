<script setup lang="ts">
import { onMounted } from 'vue'
import { useConfigStore } from '@/stores/config'

const config = useConfigStore()

onMounted(() => config.fetchSettings())

const tabs = [
  { name: 'General', keys: ['listen'] },
  { name: 'Source', keys: ['source.url', 'source.cache_path', 'source.refresh_interval', 'source.http_timeout'] },
  { name: 'Resolver', keys: ['resolver.upstream', 'resolver.network', 'resolver.concurrency', 'resolver.qps', 'resolver.timeout', 'resolver.retries', 'resolver.backoff_base', 'resolver.backoff_max', 'resolver.follow_cname', 'resolver.enable_v4', 'resolver.enable_v6'] },
  { name: 'Cache', keys: ['cache.db_path', 'cache.ttl', 'cache.failed_ttl', 'cache.vacuum_after'] },
  { name: 'Aggregation', keys: ['aggregation.enabled', 'aggregation.level', 'aggregation.v4_max_prefix', 'aggregation.v6_max_prefix'] },
  { name: 'Export', keys: ['export.dir', 'export.ipv4_file', 'export.ipv6_file'] },
  { name: 'Routing', keys: ['routing.enabled', 'routing.backend', 'routing.table_id', 'routing.iface', 'routing.nft_table', 'routing.nft_set_v4', 'routing.nft_set_v6', 'routing.state_path', 'routing.dry_run'] },
  { name: 'Scheduler', keys: ['scheduler.dlc_refresh', 'scheduler.resolve_cycle'] },
  { name: 'Logging', keys: ['logging.level', 'logging.format'] },
  { name: 'Metrics', keys: ['metrics.enabled', 'metrics.path'] },
]

function isOverridden(key: string): boolean {
  return config.settings?.overrides?.[key] !== undefined
}

async function saveOverride(key: string, value: string) {
  await config.updateOverride(key, value)
}

async function removeOverride(key: string) {
  await config.deleteOverride(key)
}
</script>

<template>
  <n-card title="Configuration">
    <n-spin v-if="config.loading" />
    <n-empty v-else-if="!config.settings" description="Failed to load settings" />
    <n-tabs v-else type="line">
      <n-tab-pane v-for="tab in tabs" :key="tab.name" :name="tab.name" :tab="tab.name">
        <n-form label-placement="left" label-width="180">
          <n-form-item v-for="key in tab.keys" :key="key" :label="key">
            <n-input-group>
              <n-input
                :value="config.settings.overrides[key] ?? (config.settings.config as any)[key] ?? ''"
                :placeholder="(config.settings.defaults as any)[key]?.toString() ?? ''"
                @update:value="(v: string) => saveOverride(key, v)"
              />
              <n-button v-if="isOverridden(key)" type="warning" @click="removeOverride(key)">Reset</n-button>
            </n-input-group>
          </n-form-item>
        </n-form>
      </n-tab-pane>
    </n-tabs>
  </n-card>
</template>
