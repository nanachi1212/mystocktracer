import { describe, expect, it } from 'vitest';
import { llmLocalConnectionError, llmLocalPresetFor, llmLocalPresets, llmProviderDefaultModel,
  llmProviderDefinition, llmProviderName, llmProviders } from './llm-providers';

describe('model provider configuration contract', () => {
  it('keeps the supported provider identity, endpoint and protocol', () => {
    const expected = [
      ['moonshot', 'https://api.moonshot.cn/v1'],
      ['minimax', 'https://api.minimaxi.com/v1'],
      ['zhipu', 'https://open.bigmodel.cn/api/paas/v4'],
      ['qwen', 'https://dashscope.aliyuncs.com/compatible-mode/v1'],
      ['siliconflow', 'https://api.siliconflow.cn/v1'],
    ];
    for (const [id, baseURL] of expected) {
      expect(llmProviderDefinition(id)).toMatchObject({ id, baseURL, apiMode: 'chat_completions' });
    }
    expect(llmProviderDefinition('anthropic').apiMode).toBe('anthropic_messages');
    expect(llmProviderDefaultModel('deepseek')).toBe('deepseek-chat');
    expect(llmProviderName('moonshot')).toContain('Kimi');
    expect(llmProviders.map(({ id }) => id)).toContain('custom');
  });

  it('sends an unknown provider through the custom connection contract', () => {
    expect(llmProviderDefinition('synthetic-unknown')).toMatchObject({ id: 'custom', baseURL: '', defaultModel: '' });
    expect(llmProviderDefaultModel('synthetic-unknown')).toBe('');
    expect(llmProviderName('synthetic-unknown')).toBe('synthetic-unknown');
  });

  it('recognizes loopback presets by origin and gives useful local errors', () => {
    expect(llmLocalPresets.map(({ id, provider }) => [id, provider])).toEqual([['ollama', 'custom'], ['lmstudio', 'custom']]);
    expect(llmLocalPresetFor('http://127.0.0.1:11434/other')?.id).toBe('ollama');
    expect(llmLocalPresetFor('http://127.0.0.1:1234/v1')?.id).toBe('lmstudio');
    expect(llmLocalPresetFor('file:///unsafe')).toBeUndefined();
    expect(llmLocalConnectionError('http://127.0.0.1:11434/v1', new Error('network'))).toContain('Ollama');
    expect(llmLocalConnectionError('http://127.0.0.1:1234/v1', new Error('network'))).toContain('LM Studio');
    expect(llmLocalConnectionError('https://model.example', new Error('synthetic failure'))).toBe('synthetic failure');
  });
});
