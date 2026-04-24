import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CacheChart from '../CacheChart.vue'

describe('CacheChart', () => {
  it('renders domain distribution bars', () => {
    const wrapper = mount(CacheChart, {
      props: {
        stats: {
          domains_valid: 80,
          domains_failed: 10,
          domains_nxdomain: 10,
        },
      },
    })

    expect(wrapper.text()).toContain('Valid')
    expect(wrapper.text()).toContain('Failed')
    expect(wrapper.text()).toContain('NXDomain')
    expect(wrapper.text()).toContain('80')
    expect(wrapper.text()).toContain('10')
  })

  it('computes percentages correctly', () => {
    const wrapper = mount(CacheChart, {
      props: {
        stats: {
          domains_valid: 50,
          domains_failed: 25,
          domains_nxdomain: 25,
        },
      },
    })

    expect(wrapper.text()).toContain('50 (50%)')
    expect(wrapper.text()).toContain('25 (25%)')
  })

  it('handles zero total gracefully', () => {
    const wrapper = mount(CacheChart, {
      props: {
        stats: {
          domains_valid: 0,
          domains_failed: 0,
          domains_nxdomain: 0,
        },
      },
    })

    expect(wrapper.text()).toContain('0 (0%)')
  })
})
