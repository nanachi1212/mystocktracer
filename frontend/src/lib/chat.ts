export type ChatRole = 'user' | 'assistant';
export type ChatMessage = { id: string; role: ChatRole; content: string; created_at: string; error?: boolean };
export type ChatConversation = { id: string; title: string; session_id?: string; messages: ChatMessage[]; created_at: string; updated_at: string };

export function createChatID(prefix = 'chat') {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
  return `${prefix}-${random}`;
}

export function createChatConversation(now = new Date().toISOString()): ChatConversation {
  return { id: createChatID('conversation'), title: '新對話', messages: [], created_at: now, updated_at: now };
}

export function deriveChatTitle(input: string): string {
  const normalized = input.trim().replace(/\s+/g, ' ');
  if (!normalized) return '新對話';
  const codePoints = Array.from(normalized);
  return codePoints.length > 24 ? `${codePoints.slice(0, 24).join('')}…` : normalized;
}

export function storeableConversations(conversations: ChatConversation[]): ChatConversation[] {
  return Array.from(conversations)
    .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
    .slice(0, 30)
    .map((conversation) => ({ ...conversation, messages: conversation.messages.slice(-100) }));
}

export function clearAgentSessionIDs(conversations: ChatConversation[]): ChatConversation[] {
  return conversations.map(({ session_id: _session, ...conversation }) => conversation);
}

function object(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function validMessage(value: unknown): value is ChatMessage {
  if (!object(value) || (value.role !== 'user' && value.role !== 'assistant')) return false;
  return typeof value.id === 'string' && typeof value.content === 'string' && typeof value.created_at === 'string' &&
    (value.error === undefined || typeof value.error === 'boolean');
}

function savedConversation(value: unknown): ChatConversation | null {
  if (!object(value) || !Array.isArray(value.messages) || !value.messages.every(validMessage)) return null;
  if ([value.id, value.title, value.created_at, value.updated_at].some((field) => typeof field !== 'string')) return null;
  const historicalSession = value.session_id ?? value.hermes_session_id;
  if (historicalSession !== undefined && typeof historicalSession !== 'string') return null;
  return {
    id: value.id as string, title: value.title as string,
    created_at: value.created_at as string, updated_at: value.updated_at as string,
    messages: value.messages,
    ...(historicalSession ? { session_id: historicalSession } : {}),
  };
}

export function parseStoredConversations(raw: string | null): ChatConversation[] {
  if (!raw) return [];
  let document: unknown;
  try { document = JSON.parse(raw); } catch { return []; }
  if (!Array.isArray(document)) return [];
  const accepted: ChatConversation[] = [];
  for (const entry of document) {
    const conversation = savedConversation(entry);
    if (conversation) accepted.push(conversation);
  }
  return storeableConversations(accepted);
}
