<script setup lang="ts">
import { onMounted, ref, h, watch } from 'vue'
import { useSourcesStore } from '@/stores/sources'
import { useMessage, NButton } from 'naive-ui'
import type { SourceConfig } from '@/api/types'

const store = useSourcesStore()
const message = useMessage()

const showModal = ref(false)
const editing = ref<SourceConfig | null>(null)

onMounted(() => store.fetchSources())

const providerOptions = [
  { label: 'Plaintext (domains/IPs)', value: 'plaintext' },
  { label: 'V2fly Geosite', value: 'v2flygeosite' },
  { label: 'V2fly GeoIP', value: 'v2flygeoip' },
  { label: 'IPverse (country blocks)', value: 'ipverse' },
  { label: 'MaxMind MMDB', value: 'mmdb' },
]

const countryOptions = [
  { label: 'Argentina (AR)', value: 'ar' },
  { label: 'Australia (AU)', value: 'au' },
  { label: 'Austria (AT)', value: 'at' },
  { label: 'Bangladesh (BD)', value: 'bd' },
  { label: 'Belgium (BE)', value: 'be' },
  { label: 'Brazil (BR)', value: 'br' },
  { label: 'Canada (CA)', value: 'ca' },
  { label: 'Chile (CL)', value: 'cl' },
  { label: 'China (CN)', value: 'cn' },
  { label: 'Colombia (CO)', value: 'co' },
  { label: 'Czech Republic (CZ)', value: 'cz' },
  { label: 'Denmark (DK)', value: 'dk' },
  { label: 'Egypt (EG)', value: 'eg' },
  { label: 'Finland (FI)', value: 'fi' },
  { label: 'France (FR)', value: 'fr' },
  { label: 'Germany (DE)', value: 'de' },
  { label: 'Greece (GR)', value: 'gr' },
  { label: 'Hungary (HU)', value: 'hu' },
  { label: 'India (IN)', value: 'in' },
  { label: 'Indonesia (ID)', value: 'id' },
  { label: 'Iran (IR)', value: 'ir' },
  { label: 'Ireland (IE)', value: 'ie' },
  { label: 'Israel (IL)', value: 'il' },
  { label: 'Italy (IT)', value: 'it' },
  { label: 'Japan (JP)', value: 'jp' },
  { label: 'Kenya (KE)', value: 'ke' },
  { label: 'Malaysia (MY)', value: 'my' },
  { label: 'Mexico (MX)', value: 'mx' },
  { label: 'Netherlands (NL)', value: 'nl' },
  { label: 'Nigeria (NG)', value: 'ng' },
  { label: 'Norway (NO)', value: 'no' },
  { label: 'Pakistan (PK)', value: 'pk' },
  { label: 'Philippines (PH)', value: 'ph' },
  { label: 'Poland (PL)', value: 'pl' },
  { label: 'Portugal (PT)', value: 'pt' },
  { label: 'Romania (RO)', value: 'ro' },
  { label: 'Russia (RU)', value: 'ru' },
  { label: 'Saudi Arabia (SA)', value: 'sa' },
  { label: 'Singapore (SG)', value: 'sg' },
  { label: 'South Africa (ZA)', value: 'za' },
  { label: 'South Korea (KR)', value: 'kr' },
  { label: 'Spain (ES)', value: 'es' },
  { label: 'Sweden (SE)', value: 'se' },
  { label: 'Switzerland (CH)', value: 'ch' },
  { label: 'Thailand (TH)', value: 'th' },
  { label: 'Turkey (TR)', value: 'tr' },
  { label: 'Ukraine (UA)', value: 'ua' },
  { label: 'United Kingdom (GB)', value: 'gb' },
  { label: 'United States (US)', value: 'us' },
  { label: 'Vietnam (VN)', value: 'vn' },
].sort((a, b) => a.label.localeCompare(b.label))

function resetConfig(provider: string) {
  if (!editing.value) return
  if (provider === 'ipverse') {
    editing.value.config = { base_url: 'https://ipverse.net/ipblocks/data/countries/{country}.zone', countries: [] }
  } else if (provider === 'plaintext') {
    editing.value.config = { type: 'domains', file: '' }
  } else if (provider === 'v2flygeosite') {
    editing.value.config = { url: '', cache_path: '' }
  } else if (provider === 'v2flygeoip') {
    editing.value.config = { url: '', cache_path: '' }
  } else if (provider === 'mmdb') {
    editing.value.config = { file: '', url: '', countries: [] }
  }
}

watch(() => editing.value?.provider, (provider) => {
  if (provider) resetConfig(provider)
})

function openAdd() {
  editing.value = {
    id: '',
    provider: 'plaintext',
    prefix: '',
    enabled: true,
    config: { type: 'domains', file: '' },
  }
  showModal.value = true
}

async function handleSave() {
  if (!editing.value) return
  if (!editing.value.id || !editing.value.prefix) {
    message.error('ID and prefix are required')
    return
  }
  const payload = { ...editing.value }
  if (payload.provider === 'ipverse') {
    payload.config = {
      base_url: payload.config.base_url,
      countries: payload.config.countries || [],
    }
  } else if (payload.provider === 'mmdb') {
    const cfg: Record<string, unknown> = {}
    if (payload.config.file) cfg.file = payload.config.file
    if (payload.config.url) cfg.url = payload.config.url
    const countries = payload.config.countries as string[]
    if (countries?.length > 0) cfg.countries = countries
    payload.config = cfg
  }
  try {
    await store.addSource(payload)
    message.success('Source added')
    showModal.value = false
  } catch (e) {
    message.error('Failed to add source')
  }
}

async function handleDelete(id: string) {
  try {
    await store.removeSource(id)
    message.success('Source deleted')
  } catch (e) {
    message.error('Failed to delete source')
  }
}

async function handleRefresh(id: string) {
  try {
    await store.reloadSource(id)
    message.success('Source refreshed')
  } catch (e) {
    message.error('Failed to refresh source')
  }
}

const columns = [
  { title: 'ID', key: 'id' },
  { title: 'Provider', key: 'provider' },
  { title: 'Prefix', key: 'prefix' },
  { title: 'Categories', key: 'categories', render: (row: any) => row.categories?.length ?? 0 },
  { title: 'Status', key: 'enabled', render: (row: any) => row.enabled ? 'Enabled' : 'Disabled' },
  { title: 'Actions', key: 'actions', render: (row: any) => h('div', { class: 'flex gap-2' }, [
    h(NButton, { size: 'small', onClick: () => handleRefresh(row.id) }, () => 'Refresh'),
    h(NButton, { size: 'small', type: 'error', onClick: () => handleDelete(row.id) }, () => 'Delete'),
  ]) },
]
</script>

<template>
  <div class="space-y-4">
    <n-card title="Sources">
      <template #header-extra>
        <n-button type="primary" @click="openAdd">Add Source</n-button>
      </template>
      <n-spin v-if="store.loading" />
      <n-empty v-else-if="store.sources.length === 0" description="No sources configured" />
      <n-data-table
        v-else
        :columns="columns"
        :data="store.sources"
      />
    </n-card>

    <n-modal v-model:show="showModal" title="Add Source" preset="card" style="width: 500px">
      <n-form v-if="editing" :model="editing">
        <n-form-item label="ID" required>
          <n-input v-model:value="editing.id" placeholder="my-source" />
        </n-form-item>
        <n-form-item label="Prefix" required>
          <n-input v-model:value="editing.prefix" placeholder="corp" />
        </n-form-item>
        <n-form-item label="Provider" required>
          <n-select v-model:value="editing.provider" :options="providerOptions" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'plaintext'" label="Type">
          <n-select v-model:value="editing.config.type" :options="[
            { label: 'Domains', value: 'domains' },
            { label: 'IPs', value: 'ips' },
          ]" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'plaintext'" label="File Path">
          <n-input v-model:value="editing.config.file" placeholder="/var/lib/d2ip/sources/mylist.txt" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeosite'" label="URL">
          <n-input v-model:value="editing.config.url" placeholder="https://github.com/..." />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeosite'" label="Cache Path">
          <n-input v-model:value="editing.config.cache_path" placeholder="/var/lib/d2ip/dlc.dat" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeoip'" label="URL">
          <n-input v-model:value="editing.config.url" placeholder="https://github.com/v2fly/geoip/releases/latest/download/geoip.dat" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'v2flygeoip'" label="Cache Path">
          <n-input v-model:value="editing.config.cache_path" placeholder="/var/lib/d2ip/geoip.dat" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'ipverse'" label="Base URL">
          <n-input v-model:value="editing.config.base_url" placeholder="https://ipverse.net/ipblocks/data/countries/{country}.zone" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'ipverse'" label="Countries" required>
          <n-select
            v-model:value="editing.config.countries"
            multiple
            filterable
            :options="countryOptions"
            placeholder="Select countries..."
          />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'mmdb'" label="MMDB File Path">
          <n-input v-model:value="editing.config.file" placeholder="/var/lib/d2ip/GeoLite2-Country.mmdb" />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'mmdb'" label="Download URL (optional)">
          <n-input v-model:value="editing.config.url" placeholder="https://download.maxmind.com/..." />
        </n-form-item>
        <n-form-item v-if="editing.provider === 'mmdb'" label="Countries Whitelist (optional)">
          <n-select
            v-model:value="editing.config.countries"
            multiple
            filterable
            :options="countryOptions"
            placeholder="Select countries to include (empty = all)..."
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-button @click="showModal = false">Cancel</n-button>
        <n-button type="primary" @click="handleSave">Save</n-button>
      </template>
    </n-modal>
  </div>
</template>
