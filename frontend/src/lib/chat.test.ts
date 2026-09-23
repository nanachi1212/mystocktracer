import { describe, expect, it } from 'vitest';
import { clearAgentSessionIDs, createChatConversation, deriveChatTitle, parseStoredConversations, storeableConversations, type ChatConversation } from './chat';

const conversation = (id: number): ChatConversation => ({ id: String(id), title: `對話 ${id}`, messages: [],
  created_at: '2026-09-01T00:00:00Z', updated_at: new Date(Date.UTC(2026, 8, 1, 0, id)).toISOString() });

describe('local conversation storage contract', () => {
  it('creates a fresh conversation and bounds Unicode titles', () => {
    const fresh = createChatConversation('2026-09-23T00:00:00Z');
    expect(fresh.id).toMatch(/^conversation-/);
    expect(fresh.messages).toEqual([]);
    expect(deriveChatTitle(' \n ')).toBe('新對話');
    expect(deriveChatTitle('  台股\n 研究  ')).toBe('台股 研究');
    expect(Array.from(deriveChatTitle('測'.repeat(35)))).toHaveLength(25);
  });

  it('accepts old session IDs while rejecting malformed saved entries', () => {
    const raw = JSON.stringify([{ ...conversation(1), hermes_session_id: 'historical-session' },
      { ...conversation(2), messages: [{ id: 'bad', role: 'system', content: 'x', created_at: 'now' }] },
      { ...conversation(3), session_id: 17 }, conversation(4)]);
    expect(parseStoredConversations(raw).map(({ id }) => id)).toEqual(['4', '1']);
    expect(parseStoredConversations(raw)[1].session_id).toBe('historical-session');
    expect(parseStoredConversations('{broken')).toEqual([]);
    expect(parseStoredConversations(null)).toEqual([]);
  });

  it('stores the newest thirty histories and newest hundred messages without mutating the input', () => {
    const oldest = conversation(0);
    oldest.messages = Array.from({ length: 110 }, (_, id) => ({ id: String(id), role: 'user' as const,
      content: `合成內容 ${id}`, created_at: oldest.created_at }));
    const input = [oldest, ...Array.from({ length: 34 }, (_, id) => conversation(id + 1))];
    const retained = storeableConversations(input);
    expect(retained).toHaveLength(30);
    expect(retained[0].id).toBe('34');
    const [bounded] = storeableConversations([oldest]);
    expect(bounded.messages).toHaveLength(100);
    expect(bounded.messages[0].id).toBe('10');
    expect(oldest.messages).toHaveLength(110);
  });

  it('drops runtime sessions without touching saved messages', () => {
    const current = { ...conversation(1), session_id: 'stale-session', messages: [{ id: 'message', role: 'assistant' as const,
      content: '保留資料', created_at: '2026-09-01T00:00:00Z' }] };
    expect(clearAgentSessionIDs([current])).toEqual([{ ...conversation(1), messages: current.messages }]);
    expect(current.session_id).toBe('stale-session');
  });
});
