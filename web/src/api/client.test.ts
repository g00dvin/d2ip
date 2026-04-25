import { describe, it, expect } from 'vitest'
import { client } from './client'

describe('API Client', () => {
  it('should create axios instance with correct defaults', () => {
    expect(client.defaults.baseURL).toBe('/')
    expect(client.defaults.timeout).toBe(15000)
    expect(client.defaults.headers['Content-Type']).toBe('application/json')
  })

  it('should extract error message from response data', async () => {
    const errorResponse = {
      response: { data: { error: 'category already exists: geosite:ru' } },
      message: 'Request failed with status code 409',
    }

    const handlers = (client.interceptors.response as any).handlers
    expect(handlers).toBeDefined()
    expect(handlers.length).toBeGreaterThan(0)

    try {
      await handlers[0].rejected(errorResponse)
    } catch (e) {
      expect(e).toBeInstanceOf(Error)
      expect((e as Error).message).toBe('category already exists: geosite:ru')
    }
  })

  it('should fallback to network error message when no response data', async () => {
    const networkError = {
      response: null,
      message: 'Network Error',
    }

    const handlers = (client.interceptors.response as any).handlers
    try {
      await handlers[0].rejected(networkError)
    } catch (e) {
      expect(e).toBeInstanceOf(Error)
      expect((e as Error).message).toBe('Network Error')
    }
  })

  it('should fallback to generic message when response data has no error field', async () => {
    const genericError = {
      response: { data: {} },
      message: 'Something went wrong',
    }

    const handlers = (client.interceptors.response as any).handlers
    try {
      await handlers[0].rejected(genericError)
    } catch (e) {
      expect(e).toBeInstanceOf(Error)
      expect((e as Error).message).toBe('Something went wrong')
    }
  })

  it('should pass through successful responses', async () => {
    const response = { data: { status: 'ok' }, status: 200, statusText: 'OK', headers: {}, config: {} as any }

    const handlers = (client.interceptors.response as any).handlers
    const result = await handlers[0].fulfilled(response)
    expect(result).toBe(response)
  })
})
