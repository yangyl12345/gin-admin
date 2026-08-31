import { describe, expect, it } from 'vitest'
import { api, getSessionKey, isIndexingStatus, isRetryableRunStatus, KEY_STORAGE, parseSSEBlock, setSessionKey, timelineLabel } from './api'

class MemoryStorage implements Storage {
  private values = new Map<string, string>()
  get length() { return this.values.size }
  clear() { this.values.clear() }
  getItem(key: string) { return this.values.get(key) ?? null }
  key(index: number) { return [...this.values.keys()][index] ?? null }
  removeItem(key: string) { this.values.delete(key) }
  setItem(key: string, value: string) { this.values.set(key, value) }
}

describe('Agent API session key', () => {
  it('uses sessionStorage only and clears blank values', () => {
    const session = new MemoryStorage()
    setSessionKey('  secret  ', session)
    expect(getSessionKey(session)).toBe('secret')
    expect(session.getItem(KEY_STORAGE)).toBe('secret')
    setSessionKey(' ', session)
    expect(getSessionKey(session)).toBe('')
  })
})

describe('SSE parser', () => {
  it('parses replayable event ids and payloads', () => {
    expect(parseSSEBlock('id: 42\nevent: step.completed\ndata: {"role":"reviewer"}')).toEqual({ id: 42, event: 'step.completed', data: { role: 'reviewer' } })
  })

  it('ignores heartbeat comments', () => {
    expect(parseSSEBlock(': heartbeat')).toBeNull()
  })

  it('keeps the durable run metadata used by the SSE timeline', () => {
    const event = parseSSEBlock('id: 8\nevent: step.completed\ndata: {"run_id":"run_1","time":"2026-08-30T00:00:00Z","role":"reviewer"}')
    expect(event).toEqual({ id: 8, event: 'step.completed', data: { run_id: 'run_1', time: '2026-08-30T00:00:00Z', role: 'reviewer' } })
    expect(timelineLabel(event!)).toBe('#8 step.completed · reviewer')
  })
})

describe('Agent UI state recovery', () => {
  it('recognizes indexing and retryable terminal states', () => {
    expect(isIndexingStatus('pending')).toBe(true)
    expect(isIndexingStatus('ready')).toBe(false)
    expect(isRetryableRunStatus('failed_review')).toBe(true)
    expect(isRetryableRunStatus('completed')).toBe(false)
  })

  it('surfaces a wrapped API error without retaining response state', async () => {
    const originalFetch = globalThis.fetch
    globalThis.fetch = async () => new Response(JSON.stringify({ success: false, error: { detail: 'Agent is disabled' } }), { status: 503, headers: { 'Content-Type': 'application/json' } })
    await expect(api('/knowledge-bases')).rejects.toThrow('Agent is disabled')
    globalThis.fetch = originalFetch
  })
})
