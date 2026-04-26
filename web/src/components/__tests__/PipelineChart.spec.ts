import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PipelineChart from '../PipelineChart.vue'

describe('PipelineChart', () => {
  it('renders history runs', () => {
    const history = [
      { run_id: 1, resolved: 50, duration: 1_000_000_000, domains: 50 },
      { run_id: 2, resolved: 100, duration: 2_000_000_000, domains: 100 },
    ]
    const wrapper = mount(PipelineChart, { props: { history } })
    expect(wrapper.text()).toContain('#2')
    expect(wrapper.text()).toContain('100 resolved')
    expect(wrapper.text()).toContain('2.0s')
  })

  it('computes bar width relative to max resolved', () => {
    const history = [{ run_id: 1, resolved: 50, duration: 1e9, domains: 50 }]
    const wrapper = mount(PipelineChart, { props: { history } })
    const bar = wrapper.find('.bg-blue-500')
    expect(bar.attributes('style')).toContain('width: 100%')
  })

  it('limits to last 20 runs', () => {
    const history = Array.from({ length: 25 }, (_, i) => ({
      run_id: i + 1,
      resolved: i + 1,
      duration: 1e9,
      domains: i + 1,
    }))
    const wrapper = mount(PipelineChart, { props: { history } })
    const runs = wrapper.findAll('.gap-3')
    expect(runs.length).toBe(20)
  })
})
