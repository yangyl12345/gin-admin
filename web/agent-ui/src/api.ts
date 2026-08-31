export const API_BASE = '/api/v1/agent'
export const KEY_STORAGE = 'gin-admin.agent.api-key'

export type Envelope<T> = { success: boolean; data: T; total?: number; error?: { detail?: string } }
export type Citation = { document_id: string; document_name: string; chunk_id: string; line_start: number; line_end: number; quote: string; score: number }
export type Message = { id: string; role: 'user' | 'assistant'; content: string; created_at: string; citations?: Citation[] }
export type Step = { id: string; role: string; model?: string; status: string; summary?: string; duration_ms: number; input_tokens: number; output_tokens: number; total_tokens: number }
export type Run = { id: string; status: string; revision_count: number; input_tokens: number; output_tokens: number; total_tokens: number; error_summary?: string; steps?: Step[]; final_message?: Message }
export type RunEvent = { id: number; event: string; data: Record<string, unknown> }

export function isIndexingStatus(status: string): boolean {
  return status === 'pending' || status === 'processing'
}

export function isRetryableRunStatus(status: string): boolean {
  return status === 'failed' || status === 'failed_review' || status === 'interrupted'
}

export function timelineLabel(event: RunEvent): string {
  const role = typeof event.data.role === 'string' ? ` · ${event.data.role}` : ''
  return `#${event.id} ${event.event}${role}`
}

export function getSessionKey(storage: Storage = sessionStorage): string {
  return storage.getItem(KEY_STORAGE) ?? ''
}

export function setSessionKey(value: string, storage: Storage = sessionStorage): void {
  const clean = value.trim()
  if (clean) storage.setItem(KEY_STORAGE, clean)
  else storage.removeItem(KEY_STORAGE)
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  const key = getSessionKey()
  if (key) headers.set('Authorization', `Bearer ${key}`)
  if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers })
  const body = (await response.json()) as Envelope<T>
  if (!response.ok || !body.success) throw new Error(body.error?.detail || `请求失败 (${response.status})`)
  return body.data
}

export function parseSSEBlock(block: string): RunEvent | null {
  let id = 0
  let event = ''
  const data: string[] = []
  for (const line of block.split(/\r?\n/)) {
    if (line.startsWith('id:')) id = Number(line.slice(3).trim())
    else if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) data.push(line.slice(5).trimStart())
  }
  if (!event) return null
  let payload: Record<string, unknown> = {}
  if (data.length) payload = JSON.parse(data.join('\n')) as Record<string, unknown>
  return { id, event, data: payload }
}

export async function streamRun(runID: string, onEvent: (event: RunEvent) => void, after = 0): Promise<void> {
  const response = await fetch(`${API_BASE}/runs/${runID}/events?after=${after}`, { headers: { Authorization: `Bearer ${getSessionKey()}` } })
  if (!response.ok || !response.body) throw new Error(`事件流连接失败 (${response.status})`)
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value, { stream: !done })
    let split = buffer.search(/\r?\n\r?\n/)
    while (split >= 0) {
      const block = buffer.slice(0, split)
      const separator = buffer.slice(split).match(/^\r?\n\r?\n/)?.[0].length ?? 2
      buffer = buffer.slice(split + separator)
      const event = parseSSEBlock(block)
      if (event) onEvent(event)
      split = buffer.search(/\r?\n\r?\n/)
    }
    if (done) break
  }
}
