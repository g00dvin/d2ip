import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { PolicyConfig } from '@/api/types'
import * as api from '@/api/rest'

export const usePoliciesStore = defineStore('policies', () => {
  const policies = ref<PolicyConfig[]>([])
  const loading = ref(false)

  async function fetchPolicies() {
    loading.value = true
    try {
      const resp = await api.getPolicies()
      policies.value = resp.policies ?? []
    } finally {
      loading.value = false
    }
  }

  async function createPolicy(policy: PolicyConfig) {
    await api.createPolicy(policy)
    await fetchPolicies()
  }

  async function updatePolicy(name: string, policy: PolicyConfig) {
    await api.updatePolicy(name, policy)
    await fetchPolicies()
  }

  async function deletePolicy(name: string) {
    await api.deletePolicy(name)
    await fetchPolicies()
  }

  return { policies, loading, fetchPolicies, createPolicy, updatePolicy, deletePolicy }
})
