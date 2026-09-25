export type LLMAPIMode = 'chat_completions' | 'codex_responses' | 'anthropic_messages';
export type LLMProviderDefinition = { id: string; label: string; baseURL: string; defaultModel: string; apiMode: LLMAPIMode };
export type LLMLocalPreset = LLMProviderDefinition & { provider: 'custom' };

// Default model per provider; refresh the Anthropic default when its model retires.
const catalog: [string, string, string, string][] = [
  ['openai', 'OpenAI', 'https://api.openai.com/v1', 'gpt-4o-mini'],
  ['deepseek', 'DeepSeek', 'https://api.deepseek.com', 'deepseek-chat'],
  ['moonshot', 'Kimi（月之暗面）', 'https://api.moonshot.cn/v1', 'moonshot-v1-8k'],
  ['minimax', 'MiniMax', 'https://api.minimaxi.com/v1', 'MiniMax-Text-01'],
  ['zhipu', '智譜 GLM', 'https://open.bigmodel.cn/api/paas/v4', 'glm-4-plus'],
  ['qwen', '通義千問（百煉）', 'https://dashscope.aliyuncs.com/compatible-mode/v1', 'qwen-plus'],
  ['siliconflow', '矽基流動', 'https://api.siliconflow.cn/v1', ''],
  ['anthropic', 'Anthropic', 'https://api.anthropic.com', 'claude-haiku-4-5'],
  ['custom', 'OpenAI 相容介面', '', ''],
];
export const llmProviders: LLMProviderDefinition[] = catalog.map(([id, label, baseURL, defaultModel]) => ({ id, label, baseURL, defaultModel, apiMode: id === 'anthropic' ? 'anthropic_messages' : 'chat_completions' }));
export const llmLocalPresets: LLMLocalPreset[] = [
  { id: 'ollama', label: 'Ollama', provider: 'custom', baseURL: 'http://127.0.0.1:11434/v1', defaultModel: '', apiMode: 'chat_completions' },
  { id: 'lmstudio', label: 'LM Studio', provider: 'custom', baseURL: 'http://127.0.0.1:1234/v1', defaultModel: '', apiMode: 'chat_completions' },
];
const lookup = (id: string) => llmProviders.find((item) => item.id === id);
export const llmProviderDefinition = (id: string): LLMProviderDefinition => lookup(id) ?? llmProviders[llmProviders.length - 1];
export const llmProviderName = (id: string) => lookup(id)?.label ?? id;
export const llmProviderDefaultModel = (id: string) => lookup(id)?.defaultModel ?? '';
export function llmLocalPresetFor(input: string): LLMLocalPreset | undefined {
  try { const url = new URL(input.trim()); return llmLocalPresets.find((item) => new URL(item.baseURL).origin === url.origin); }
  catch { return undefined; }
}
export function llmLocalConnectionError(baseURL: string, reason: unknown) {
  const local = llmLocalPresetFor(baseURL);
  if (local?.id === 'ollama') return '無法連線 Ollama，請確認本地服務已啟動';
  if (local?.id === 'lmstudio') return '無法連線 LM Studio，請確認 Local Server 已啟動';
  return reason instanceof Error && reason.message ? reason.message : '模型連線測試失敗';
}