export type LLMAPIMode = 'chat_completions' | 'codex_responses' | 'anthropic_messages';

export type LLMProviderDefinition = {
	id: string;
	label: string;
	baseURL: string;
	defaultModel: string;
	apiMode: LLMAPIMode;
};

export type LLMLocalPreset = LLMProviderDefinition & { provider: 'custom' };

export const llmProviders: LLMProviderDefinition[] = [
	{ id: 'openai', label: 'OpenAI', baseURL: 'https://api.openai.com/v1', defaultModel: 'gpt-4o-mini', apiMode: 'chat_completions' },
	{ id: 'deepseek', label: 'DeepSeek', baseURL: 'https://api.deepseek.com', defaultModel: 'deepseek-chat', apiMode: 'chat_completions' },
	{ id: 'moonshot', label: 'Kimi（月之暗面）', baseURL: 'https://api.moonshot.cn/v1', defaultModel: 'moonshot-v1-8k', apiMode: 'chat_completions' },
	{ id: 'minimax', label: 'MiniMax', baseURL: 'https://api.minimaxi.com/v1', defaultModel: 'MiniMax-Text-01', apiMode: 'chat_completions' },
	{ id: 'zhipu', label: '智谱 GLM', baseURL: 'https://open.bigmodel.cn/api/paas/v4', defaultModel: 'glm-4-plus', apiMode: 'chat_completions' },
	{ id: 'qwen', label: '通义千问（百炼）', baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1', defaultModel: 'qwen-plus', apiMode: 'chat_completions' },
	{ id: 'siliconflow', label: '硅基流动', baseURL: 'https://api.siliconflow.cn/v1', defaultModel: '', apiMode: 'chat_completions' },
	{ id: 'anthropic', label: 'Anthropic', baseURL: 'https://api.anthropic.com', defaultModel: 'claude-3-5-haiku-latest', apiMode: 'anthropic_messages' },
	{ id: 'custom', label: 'OpenAI 兼容接口', baseURL: '', defaultModel: '', apiMode: 'chat_completions' },
];

export const llmLocalPresets: LLMLocalPreset[] = [
	{ id: 'ollama', label: 'Ollama', provider: 'custom', baseURL: 'http://127.0.0.1:11434/v1', defaultModel: '', apiMode: 'chat_completions' },
	{ id: 'lmstudio', label: 'LM Studio', provider: 'custom', baseURL: 'http://127.0.0.1:1234/v1', defaultModel: '', apiMode: 'chat_completions' },
];

const providersByID = new Map(llmProviders.map((provider) => [provider.id, provider]));

export function llmProviderDefinition(provider: string): LLMProviderDefinition {
	return providersByID.get(provider) || providersByID.get('custom')!;
}

export function llmProviderName(provider: string): string {
	return providersByID.get(provider)?.label || provider;
}

export function llmProviderDefaultModel(provider: string): string {
	return providersByID.get(provider)?.defaultModel || '';
}

export function llmLocalPresetFor(baseURL: string): LLMLocalPreset | undefined {
	const value = baseURL.trim().toLowerCase();
	return llmLocalPresets.find((preset) => value.includes(new URL(preset.baseURL).host));
}

export function llmLocalConnectionError(baseURL: string, error: unknown): string {
	const preset = llmLocalPresetFor(baseURL);
	if (preset) return preset.id === 'ollama' ? '無法連線 Ollama，請確認本地服務已啟動' : '無法連線 LM Studio，請確認 Local Server 已啟動';
	return error instanceof Error && error.message ? error.message : '模型連線測試失敗';
}
