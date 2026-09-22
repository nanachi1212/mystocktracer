import type { BackendConfig } from './backend';
import { logRuntimeEvent, runtimeErrorDetails } from './runtime-log';

export type AgentStreamResult = { content: string; sessionID: string };
export type AgentStreamRequest = {
	config: BackendConfig;
	prompt: string;
	sessionID?: string;
	seedMessages?: Array<{ role: 'user' | 'assistant'; content: string }>;
	onDelta?: (content: string) => void;
	onSession?: (sessionID: string) => void;
	signal?: AbortSignal;
};

type StreamEvent = { version?: number; type?: string; id?: string; payload?: Record<string, unknown>; error?: { message?: string; code?: string } };
const protocolVersion = 1;
const handshakeTimeoutMS = 20_000;

export function buildAgentWebSocketURL(config: BackendConfig) {
	const url = new URL('/api/v1/ai/ws', config.backendUrl);
	url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
	if (config.token) url.searchParams.set('token', config.token);
	return url.toString();
}

export function streamAgentPrompt(request: AgentStreamRequest): Promise<AgentStreamResult> {
	return new Promise((resolve, reject) => {
		let socket: WebSocket;
		let settled = false;
		let sessionID = request.sessionID || '';
		let content = '';
		let timer: ReturnType<typeof setTimeout> | undefined;
		const finish = (error?: unknown) => {
			if (settled) return;
			settled = true;
			if (timer) clearTimeout(timer);
			request.signal?.removeEventListener('abort', abort);
			if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) socket.close();
			if (error) {
				if (!(error instanceof Error) || error.name !== 'AbortError') logRuntimeEvent('error', 'ai-chat', { event: 'agent_stream_failure', error: runtimeErrorDetails(error) });
				reject(error);
			} else resolve({ content, sessionID });
		};
		const send = (type: string, payload: Record<string, unknown> = {}) => socket.send(JSON.stringify({ version: protocolVersion, type, payload }));
		const arm = (message: string) => { if (timer) clearTimeout(timer); timer = setTimeout(() => finish(new Error(message)), handshakeTimeoutMS); };
		function abort() {
			if (socket.readyState === WebSocket.OPEN) send('session.interrupt', { session_id: sessionID });
			const error = new Error('AI 對話已停止'); error.name = 'AbortError'; finish(error);
		}
		try { socket = new WebSocket(buildAgentWebSocketURL(request.config)); }
		catch (error) { reject(error); return; }
		request.signal?.addEventListener('abort', abort, { once: true });
		if (request.signal?.aborted) { abort(); return; }
		arm('連線至 AI 執行環境逾時');
		socket.onmessage = (message) => {
			let event: StreamEvent;
			try { event = JSON.parse(String(message.data)) as StreamEvent; } catch { finish(new Error('AI 執行環境傳回無效訊息')); return; }
			if (event.version !== protocolVersion || typeof event.type !== 'string') { finish(new Error('AI 執行環境協定不相容')); return; }
			if (event.type === 'runtime.ready') { send('session.start', { resume_session_id: request.sessionID || undefined, seed_messages: request.seedMessages || [] }); arm('建立 AI 對話逾時'); return; }
			if (event.type === 'session.ready') {
				const value = event.payload?.session_id;
				if (typeof value !== 'string' || !value) { finish(new Error('AI 對話工作階段無效')); return; }
				sessionID = value; request.onSession?.(sessionID); send('prompt.submit', { session_id: sessionID, text: request.prompt }); arm('AI 回覆逾時'); return;
			}
			if (event.type === 'message.delta') { const value = event.payload?.content; if (typeof value === 'string') { content = value; request.onDelta?.(content); } return; }
			if (event.type === 'message.complete') { const value = event.payload?.content; if (typeof value === 'string') { content = value; request.onDelta?.(content); } finish(); return; }
			if (event.type === 'runtime.error') finish(new Error(typeof event.error?.message === 'string' ? event.error.message : 'AI 執行環境發生錯誤'));
		};
		socket.onerror = () => finish(new Error('無法連線至 AI 執行環境'));
		socket.onclose = () => { if (!settled) finish(new Error('AI 執行環境連線已中斷')); };
	});
}
