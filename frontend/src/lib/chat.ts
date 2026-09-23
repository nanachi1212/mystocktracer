export type ChatRole = 'user' | 'assistant';
export type ChatMessage = { id: string; role: ChatRole; content: string; created_at: string; error?: boolean };
export type ChatConversation = { id: string; title: string; session_id?: string; messages: ChatMessage[]; created_at: string; updated_at: string };

export function createChatID(prefix = 'chat') {
  return [prefix, globalThis.crypto?.randomUUID?.() ?? (Date.now().toString(36) + Math.random().toString(36).slice(2))].join('-');
}
export function createChatConversation(now = new Date().toISOString()): ChatConversation {
  return { id: createChatID('conversation'), title: '新對話', created_at: now, updated_at: now, messages: [] };
}
export function deriveChatTitle(value: string) {
  const text = value.trim().replace(/\s+/g, ' ');
  if (!text) return '新對話';
  const characters = [...text];
  return characters.slice(0, 24).join('') + (characters.length > 24 ? '…' : '');
}
export function storeableConversations(values: ChatConversation[]) {
  return [...values].sort((left, right) => right.updated_at.localeCompare(left.updated_at)).slice(0, 30)
    .map((item) => ({ ...item, messages: item.messages.slice(-100) }));
}
export function clearAgentSessionIDs(values: ChatConversation[]): ChatConversation[] {
  return values.map((item) => { const copy = { ...item }; delete copy.session_id; return copy; });
}
function record(value: unknown): value is Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value); }
function message(value: unknown): value is ChatMessage {
  return record(value) && ['id', 'content', 'created_at'].every((key) => typeof value[key] === 'string')
    && (value.role === 'user' || value.role === 'assistant') && (value.error === undefined || typeof value.error === 'boolean');
}
export function parseStoredConversations(raw: string | null): ChatConversation[] {
  let decoded: unknown;
  try { decoded = JSON.parse(raw || 'null'); } catch { return []; }
  if (!Array.isArray(decoded)) return [];
  const valid: ChatConversation[] = [];
  for (const value of decoded) {
    if (!record(value) || !['id', 'title', 'created_at', 'updated_at'].every((key) => typeof value[key] === 'string')
      || !Array.isArray(value.messages) || !value.messages.every(message)) continue;
    const session = value.session_id ?? value.hermes_session_id; // Read-only stored-history compatibility.
    if (session !== undefined && typeof session !== 'string') continue;
    valid.push({ id: value.id as string, title: value.title as string, created_at: value.created_at as string, updated_at: value.updated_at as string,
      messages: value.messages, ...(session ? { session_id: session as string } : {}) });
  }
  return storeableConversations(valid);
}