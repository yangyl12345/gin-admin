<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, getSessionKey, isIndexingStatus, isRetryableRunStatus, setSessionKey, streamRun, timelineLabel, type Message, type Run, type RunEvent } from './api'

type KB = { id: string; name: string; description?: string; status: string }
type Document = { id: string; original_name: string; index_status: string; error_summary?: string; size_bytes: number }
type Conversation = { id: string; knowledge_base_id: string; title: string }

const keyDraft = ref(getSessionKey())
const keyReady = ref(Boolean(keyDraft.value))
const status = ref<{ enabled: boolean; workers_running: boolean; models: Record<string, string>; embedding_model: string } | null>(null)
const knowledgeBases = ref<KB[]>([])
const selectedKB = ref('')
const documents = ref<Document[]>([])
const conversations = ref<Conversation[]>([])
const selectedConversation = ref('')
const messages = ref<Message[]>([])
const run = ref<Run | null>(null)
const events = ref<RunEvent[]>([])
const answerBuffer = ref('')
const newKBName = ref('')
const newKBDescription = ref('')
const newConversationTitle = ref('')
const question = ref('')
const lastQuestion = ref('')
const busy = ref(false)
const error = ref('')

const currentKB = computed(() => knowledgeBases.value.find((item) => item.id === selectedKB.value))
const currentConversation = computed(() => conversations.value.find((item) => item.id === selectedConversation.value))
const terminalFailure = computed(() => Boolean(run.value && isRetryableRunStatus(run.value.status)))

async function guard(task: () => Promise<void>) {
  error.value = ''
  try { await task() } catch (cause) { error.value = cause instanceof Error ? cause.message : '操作失败' }
}

async function loadStatus() {
  const response = await fetch('/api/v1/agent/status')
  const body = await response.json()
  status.value = body.data
}

async function connect() {
  setSessionKey(keyDraft.value)
  keyReady.value = Boolean(getSessionKey())
  if (!keyReady.value) return
  await guard(async () => { await Promise.all([loadKnowledgeBases(), loadConversations()]) })
}

function disconnect() {
  setSessionKey('')
  keyDraft.value = ''
  keyReady.value = false
  knowledgeBases.value = []
  conversations.value = []
  messages.value = []
}

async function loadKnowledgeBases() {
  knowledgeBases.value = await api<KB[]>('/knowledge-bases?current=1&pageSize=100')
  if (!selectedKB.value && knowledgeBases.value.length) selectedKB.value = knowledgeBases.value[0].id
  await loadDocuments()
}

async function createKnowledgeBase() {
  if (!newKBName.value.trim()) return
  await guard(async () => {
    const item = await api<KB>('/knowledge-bases', { method: 'POST', body: JSON.stringify({ name: newKBName.value, description: newKBDescription.value }) })
    newKBName.value = ''; newKBDescription.value = ''; selectedKB.value = item.id
    await loadKnowledgeBases()
  })
}

async function archiveKnowledgeBase(id: string) {
  if (!confirm('归档这个知识库？历史引用仍会保留。')) return
  await guard(async () => { await api(`/knowledge-bases/${id}`, { method: 'DELETE' }); selectedKB.value = ''; await loadKnowledgeBases() })
}

async function loadDocuments() {
  if (!selectedKB.value) { documents.value = []; return }
  documents.value = await api<Document[]>(`/knowledge-bases/${selectedKB.value}/documents?current=1&pageSize=100`)
}

async function upload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !selectedKB.value) return
  await guard(async () => {
    busy.value = true
    const form = new FormData(); form.set('file', file)
    await api(`/knowledge-bases/${selectedKB.value}/documents`, { method: 'POST', body: form })
    await loadDocuments(); watchIndexing()
  })
  busy.value = false; input.value = ''
}

function watchIndexing() {
  const timer = window.setInterval(async () => {
    try {
      await loadDocuments()
      if (!documents.value.some((item) => isIndexingStatus(item.index_status))) window.clearInterval(timer)
    } catch { window.clearInterval(timer) }
  }, 1500)
}

async function reindex(id: string) {
  await guard(async () => { await api(`/documents/${id}/reindex`, { method: 'POST' }); await loadDocuments(); watchIndexing() })
}

async function archiveDocument(id: string) {
  if (!confirm('归档这个文档？')) return
  await guard(async () => { await api(`/documents/${id}`, { method: 'DELETE' }); await loadDocuments() })
}

async function loadConversations() {
  conversations.value = await api<Conversation[]>('/conversations?current=1&pageSize=100')
}

async function createConversation() {
  if (!selectedKB.value) return
  await guard(async () => {
    const item = await api<Conversation>('/conversations', { method: 'POST', body: JSON.stringify({ knowledge_base_id: selectedKB.value, title: newConversationTitle.value }) })
    newConversationTitle.value = ''; await loadConversations(); await selectConversation(item.id)
  })
}

async function selectConversation(id: string) {
  selectedConversation.value = id; run.value = null; events.value = []; answerBuffer.value = ''
  messages.value = await api<Message[]>(`/conversations/${id}/messages?current=1&pageSize=100`)
}

async function sendQuestion(content = question.value) {
  if (!selectedConversation.value || !content.trim() || busy.value) return
  await guard(async () => {
    busy.value = true; events.value = []; answerBuffer.value = ''; run.value = null
    lastQuestion.value = content.trim(); question.value = ''
    const created = await api<{ run_id: string }>(`/conversations/${selectedConversation.value}/runs`, { method: 'POST', body: JSON.stringify({ content: lastQuestion.value }) })
    await streamRun(created.run_id, (event) => {
      events.value.push(event)
      if (event.event === 'answer.delta') answerBuffer.value += String(event.data.delta ?? '')
    })
    run.value = await api<Run>(`/runs/${created.run_id}`)
    messages.value = await api<Message[]>(`/conversations/${selectedConversation.value}/messages?current=1&pageSize=100`)
  })
  busy.value = false
}

onMounted(async () => {
  await loadStatus()
  if (keyReady.value) await connect()
})
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <div><p class="eyebrow">GIN-ADMIN · AGENT</p><h1>Knowledge Studio</h1></div>
      <div class="status" :class="status?.enabled ? 'online' : 'offline'"><span></span>{{ status?.enabled ? (status.workers_running ? 'Agent ready' : 'Starting') : 'Agent disabled' }}</div>
    </header>

    <section v-if="!keyReady" class="key-card">
      <p class="eyebrow">SESSION ACCESS</p><h2>连接本地 Agent</h2>
      <p>Agent Key 只保存在当前标签页的 sessionStorage，关闭标签页后自动清除。</p>
      <div class="key-row"><input v-model="keyDraft" type="password" autocomplete="off" placeholder="输入 AGENT_API_KEY" @keyup.enter="connect" /><button @click="connect">进入工作台</button></div>
    </section>

    <template v-else>
      <div class="toolbar"><span>Embedding · {{ status?.embedding_model }}</span><button class="ghost" @click="disconnect">清除会话 Key</button></div>
      <p v-if="error" class="error">{{ error }}</p>
      <main class="workspace">
        <aside class="rail">
          <section class="panel">
            <div class="panel-title"><div><p class="eyebrow">LIBRARY</p><h2>知识库</h2></div><span>{{ knowledgeBases.length }}</span></div>
            <div class="compact-form"><input v-model="newKBName" placeholder="知识库名称" /><textarea v-model="newKBDescription" placeholder="描述（可选）"></textarea><button @click="createKnowledgeBase">新建知识库</button></div>
            <button v-for="kb in knowledgeBases" :key="kb.id" class="list-item" :class="{ selected: selectedKB === kb.id }" @click="selectedKB = kb.id; loadDocuments()">
              <span><strong>{{ kb.name }}</strong><small>{{ kb.description || '暂无描述' }}</small></span><i @click.stop="archiveKnowledgeBase(kb.id)">归档</i>
            </button>
          </section>

          <section class="panel documents">
            <div class="panel-title"><div><p class="eyebrow">SOURCES</p><h2>文档索引</h2></div></div>
            <label class="upload" :class="{ disabled: !selectedKB || busy }"><input type="file" accept=".txt,.md,text/plain,text/markdown" :disabled="!selectedKB || busy" @change="upload" />上传 TXT / Markdown</label>
            <div v-for="doc in documents" :key="doc.id" class="doc-row"><div><strong>{{ doc.original_name }}</strong><small>{{ Math.ceil(doc.size_bytes / 1024) }} KB · <b :class="`state-${doc.index_status}`">{{ doc.index_status }}</b></small><em v-if="doc.error_summary">{{ doc.error_summary }}</em></div><div><button class="mini" @click="reindex(doc.id)">重建</button><button class="mini danger" @click="archiveDocument(doc.id)">归档</button></div></div>
            <p v-if="!documents.length" class="empty">{{ currentKB ? '尚未上传文档' : '先选择知识库' }}</p>
          </section>
        </aside>

        <section class="chat-panel">
          <div class="conversation-bar">
            <select :value="selectedConversation" @change="selectConversation(($event.target as HTMLSelectElement).value)"><option value="">选择对话</option><option v-for="item in conversations" :key="item.id" :value="item.id">{{ item.title }}</option></select>
            <input v-model="newConversationTitle" placeholder="新对话标题（可选）" /><button :disabled="!selectedKB" @click="createConversation">新建对话</button>
          </div>
          <div class="chat-heading"><div><p class="eyebrow">CONVERSATION</p><h2>{{ currentConversation?.title || '选择或创建一个对话' }}</h2></div><span v-if="currentConversation">固定知识库 · {{ knowledgeBases.find(k => k.id === currentConversation?.knowledge_base_id)?.name }}</span></div>
          <div class="messages">
            <article v-for="message in messages" :key="message.id" class="message" :class="message.role"><small>{{ message.role === 'user' ? 'YOU' : 'AGENT' }}</small><div class="content">{{ message.content }}</div>
              <details v-if="message.citations?.length"><summary>{{ message.citations.length }} 条引用</summary><blockquote v-for="citation in message.citations" :key="citation.chunk_id"><b>{{ citation.document_name }} · L{{ citation.line_start }}–{{ citation.line_end }}</b><p>{{ citation.quote }}</p><span>相似度 {{ citation.score.toFixed(3) }}</span></blockquote></details>
            </article>
            <article v-if="answerBuffer && busy" class="message assistant"><small>AGENT · VERIFIED STREAM</small><div class="content">{{ answerBuffer }}</div></article>
            <p v-if="!messages.length && !busy" class="empty center">从一个有明确范围的问题开始。回答只会引用当前对话绑定的知识库。</p>
          </div>
          <div class="composer"><textarea v-model="question" :disabled="!selectedConversation || busy" placeholder="询问当前知识库…" @keydown.meta.enter.prevent="sendQuestion()"></textarea><button :disabled="!selectedConversation || !question.trim() || busy" @click="sendQuestion()">{{ busy ? '运行中…' : '发送' }}</button></div>
        </section>

        <aside class="trace-panel">
          <div class="panel-title"><div><p class="eyebrow">TRACE</p><h2>运行轨迹</h2></div><span v-if="run">{{ run.total_tokens }} tokens</span></div>
          <div v-if="events.length" class="live-events"><span v-for="event in events" :key="event.id" :class="event.event.includes('failed') ? 'failed' : ''">{{ timelineLabel(event) }}</span></div>
          <ol v-if="run?.steps?.length" class="timeline"><li v-for="step in run.steps" :key="step.id"><span :class="step.status"></span><div><strong>{{ step.role }}</strong><small>{{ step.model }} · {{ step.duration_ms }} ms</small><p>{{ step.summary }}</p><em>{{ step.input_tokens }} in / {{ step.output_tokens }} out</em></div></li></ol>
          <div v-if="run" class="run-summary"><b :class="`state-${run.status}`">{{ run.status }}</b><span>返工 {{ run.revision_count }} 次</span><span>{{ run.total_tokens }} tokens</span><p v-if="run.error_summary">{{ run.error_summary }}</p><button v-if="terminalFailure && lastQuestion" @click="sendQuestion(lastQuestion)">显式重试本问题</button></div>
          <div v-else class="role-map"><div v-for="(model, role) in status?.models" :key="role"><span>{{ role.slice(0,1).toUpperCase() }}</span><p><b>{{ role }}</b><small>{{ model }}</small></p></div></div>
        </aside>
      </main>
    </template>
  </div>
</template>
