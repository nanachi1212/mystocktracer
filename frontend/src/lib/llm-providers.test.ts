import fs from 'node:fs';
import { describe, expect, it } from 'vitest';
import { llmLocalConnectionError, llmLocalPresets, llmProviderDefaultModel, llmProviderDefinition, llmProviderName, llmProviders } from './llm-providers';

describe('LLM provider definitions', () => {
	it('includes the supported Chinese model providers', () => {
		expect(llmProviders.map((provider) => provider.id)).toEqual(expect.arrayContaining(['moonshot', 'minimax', 'zhipu', 'qwen', 'siliconflow']));
		expect(llmProviderName('moonshot')).toContain('Kimi');
		expect(llmProviderName('zhipu')).toContain('GLM');
	});

	it('provides model-discovery defaults for provider switching', () => {
		expect(llmProviderDefinition('minimax')).toMatchObject({ baseURL: 'https://api.minimaxi.com/v1', apiMode: 'chat_completions' });
		expect(llmProviderDefinition('zhipu')).toMatchObject({ baseURL: 'https://open.bigmodel.cn/api/paas/v4', apiMode: 'chat_completions' });
		expect(llmProviderDefaultModel('deepseek')).toBe('deepseek-chat');
	});

	it('falls back to custom settings for unknown providers', () => {
		expect(llmProviderDefinition('unknown')).toMatchObject({ id: 'custom', baseURL: '', defaultModel: '' });
	});

	it('provides local presets through the existing custom/OpenAI-compatible contract', () => {
		expect(llmLocalPresets).toEqual([
			expect.objectContaining({ id: 'ollama', provider: 'custom', baseURL: 'http://127.0.0.1:11434/v1', defaultModel: '', apiMode: 'chat_completions' }),
			expect.objectContaining({ id: 'lmstudio', provider: 'custom', baseURL: 'http://127.0.0.1:1234/v1', defaultModel: '', apiMode: 'chat_completions' }),
		]);
		expect(llmLocalConnectionError('http://127.0.0.1:11434/v1', new Error('fetch failed'))).toBe('無法連線 Ollama，請確認本地服務已啟動');
		expect(llmLocalConnectionError('http://127.0.0.1:1234/v1', new Error('fetch failed'))).toBe('無法連線 LM Studio，請確認 Local Server 已啟動');
	});

	it('SettingsDrawer keeps cloud/local UI and profile secret isolation wired to existing state', () => {
		const source = fs.readFileSync(new URL('../components/SettingsDrawer.tsx', import.meta.url), 'utf8');
		expect(source).toContain('雲端 API');
		expect(source).toContain('本地模型');
		expect(source).toContain("setProvider('custom')");
		expect(source).toContain('profileKeyValues[current.id]');
		expect(source).toContain('API Key 可留空');
	});
});
