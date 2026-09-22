export type ChatRole = 'user' | 'assistant';

export type ChatMessage = {
	id: string;
	role: ChatRole;
	content: string;
	created_at: string;
	error?: boolean;
};

export type ChatConversation = {
	id: string;
	title: string;
	session_id?: string;
	messages: ChatMessage[];
	created_at: string;
	updated_at: string;
};

const MAX_STORED_CONVERSATIONS = 30;
const MAX_STORED_MESSAGES = 100;

export function createChatID(prefix = 'chat') {
	const random = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
	return `${prefix}-${random}`;
}

export function createChatConversation(now = new Date().toISOString()): ChatConversation {
	return {
		id: createChatID('conversation'),
		title: '新对话',
		messages: [],
		created_at: now,
		updated_at: now,
	};
}

export function deriveChatTitle(content: string) {
	const normalized = content.replace(/\s+/g, ' ').trim();
	if (!normalized) return '新对话';
	const runes = Array.from(normalized);
	return runes.length > 24 ? `${runes.slice(0, 24).join('')}…` : normalized;
}

export function parseStoredConversations(raw: string | null): ChatConversation[] {
	if (!raw) return [];
	try {
		const parsed: unknown = JSON.parse(raw);
		if (!Array.isArray(parsed)) return [];
		return parsed
			.map(normalizeConversation)
			.filter(isConversation)
			.map((conversation) => ({ ...conversation, messages: conversation.messages.slice(-MAX_STORED_MESSAGES) }))
			.sort((a, b) => b.updated_at.localeCompare(a.updated_at))
			.slice(0, MAX_STORED_CONVERSATIONS);
	} catch {
		return [];
	}
}

export function storeableConversations(conversations: ChatConversation[]) {
	return conversations
		.map((conversation) => ({ ...conversation, messages: conversation.messages.slice(-MAX_STORED_MESSAGES) }))
		.sort((a, b) => b.updated_at.localeCompare(a.updated_at))
		.slice(0, MAX_STORED_CONVERSATIONS);
}

export function clearAgentSessionIDs(conversations: ChatConversation[]): ChatConversation[] {
	return conversations.map((conversation) => {
		if (!conversation.session_id) return conversation;
		const { session_id: _sessionID, ...next } = conversation;
		return next;
	});
}

function normalizeConversation(value: unknown): unknown {
	if (!value || typeof value !== 'object') return value;
	const legacy = value as Record<string, unknown>;
	if (typeof legacy.hermes_session_id !== 'string' || typeof legacy.session_id === 'string') return legacy;
	const { hermes_session_id: legacySessionID, ...conversation } = legacy;
	return { ...conversation, session_id: legacySessionID };
}

function isConversation(value: unknown): value is ChatConversation {
	if (!value || typeof value !== 'object') return false;
	const item = value as Partial<ChatConversation>;
	return typeof item.id === 'string'
		&& typeof item.title === 'string'
		&& typeof item.created_at === 'string'
		&& typeof item.updated_at === 'string'
		&& (item.session_id === undefined || typeof item.session_id === 'string')
		&& Array.isArray(item.messages)
		&& item.messages.every(isMessage);
}

function isMessage(value: unknown): value is ChatMessage {
	if (!value || typeof value !== 'object') return false;
	const item = value as Partial<ChatMessage>;
	return typeof item.id === 'string'
		&& (item.role === 'user' || item.role === 'assistant')
		&& typeof item.content === 'string'
		&& typeof item.created_at === 'string'
		&& (item.error === undefined || typeof item.error === 'boolean');
}
