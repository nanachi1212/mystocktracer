import { CheckCircle2, CircleAlert, LoaderCircle, PlugZap, Plus, Save, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { type AppSettings, type BackendConfig, type LLMConnectionTestResult, type LLMModelOption, type LLMProfile, requestJSON } from '../../lib/backend';
import { llmLocalConnectionError, llmLocalPresets, llmProviderDefinition, llmProviders } from '../../lib/llm-providers';

type Props = { config: BackendConfig | null; open: boolean; onSaved?: (settings: AppSettings) => void };
type PanelState = 'idle' | 'loading' | 'saving' | 'error' | 'saved';
const defaultTimeout = 300;

function normalizeProfiles(settings: AppSettings): LLMProfile[] {
	if (settings.llm_profiles?.length) return settings.llm_profiles;
	return [{ id: 'llm-default', name: settings.llm.model || 'Default', provider: settings.llm.provider, base_url: settings.llm.base_url, model: settings.llm.model, api_mode: settings.llm.api_mode, api_key: settings.llm.api_key }];
}

export function ModelSettingsPanel({ config, open, onSaved }: Props) {
	const [settings, setSettings] = useState<AppSettings | null>(null);
	const [profiles, setProfiles] = useState<LLMProfile[]>([]);
	const [activeID, setActiveID] = useState('');
	const [provider, setProvider] = useState('openai');
	const [baseURL, setBaseURL] = useState('');
	const [model, setModel] = useState('');
	const [apiMode, setAPIMode] = useState('chat_completions');
	const [profileName, setProfileName] = useState('');
	const [timeout, setTimeoutValue] = useState(defaultTimeout);
	const [profileKeyValues, setProfileKeyValues] = useState<Record<string, string>>({});
	const [clearProfileKeys, setClearProfileKeys] = useState<Set<string>>(new Set());
	const [models, setModels] = useState<LLMModelOption[]>([]);
	const [state, setState] = useState<PanelState>('idle');
	const [message, setMessage] = useState('');
	const [testResult, setTestResult] = useState<LLMConnectionTestResult | null>(null);
	const current = useMemo(() => profiles.find((item) => item.id === activeID), [profiles, activeID]);
	const runtime = settings?.agent || settings?.hermes;

	const loadProfile = (profile: LLMProfile) => {
		setProfileName(profile.name);
		setProvider(profile.provider || 'openai');
		setBaseURL(profile.base_url || llmProviderDefinition(profile.provider || 'openai').baseURL);
		setModel(profile.model || '');
		setAPIMode(profile.api_mode === 'responses' ? 'codex_responses' : profile.api_mode || 'chat_completions');
		setModels([]);
		setTestResult(null);
	};

	useEffect(() => {
		if (!open || !config) return;
		let cancelled = false;
		setState('loading'); setMessage('');
		requestJSON<{ data: AppSettings }>(config, '/api/v1/settings').then(({ data }) => {
			if (cancelled) return;
			const next = normalizeProfiles(data);
			const selected = next.find((item) => item.id === data.active_llm_profile_id) || next[0];
			setSettings(data); setProfiles(next); setActiveID(selected.id); loadProfile(selected);
			setTimeoutValue(data.llm.response_timeout_seconds || defaultTimeout);
			setProfileKeyValues({}); setClearProfileKeys(new Set()); setState('idle');
		}).catch((error) => { if (!cancelled) { setState('error'); setMessage(error instanceof Error ? error.message : '讀取模型設定失敗'); } });
		return () => { cancelled = true; };
	}, [config, open]);

	const patchCurrent = (patch: Partial<LLMProfile>) => setProfiles((items) => items.map((item) => item.id === activeID ? { ...item, ...patch } : item));
	const selectProfile = (id: string) => { const next = profiles.find((item) => item.id === id); if (next) { setActiveID(id); loadProfile(next); } };
	const addProfile = () => {
		const definition = llmProviderDefinition('openai');
		const next: LLMProfile = { id: `llm-${Date.now()}-${Math.random().toString(16).slice(2)}`, name: '新模型設定', provider: 'openai', base_url: definition.baseURL, model: definition.defaultModel, api_mode: definition.apiMode, api_key: { configured: false } };
		setProfiles((items) => [...items, next]); setActiveID(next.id); loadProfile(next);
	};
	const removeProfile = () => {
		if (profiles.length <= 1) return;
		const remaining = profiles.filter((item) => item.id !== activeID); const next = remaining[0];
		setProfiles(remaining); setActiveID(next.id); loadProfile(next);
		setProfileKeyValues((values) => { const copy = { ...values }; delete copy[activeID]; return copy; });
		setClearProfileKeys((values) => { const copy = new Set(values); copy.delete(activeID); return copy; });
	};
	const changeProvider = (next: string) => {
		const definition = llmProviderDefinition(next);
		setProvider(next); setBaseURL(definition.baseURL); setModel(definition.defaultModel); setAPIMode(definition.apiMode);
		patchCurrent({ provider: next, base_url: definition.baseURL, model: definition.defaultModel, api_mode: definition.apiMode }); setModels([]);
	};
	const applyLocalPreset = (preset: typeof llmLocalPresets[number]) => {
		setProvider('custom'); setBaseURL(preset.baseURL); setModel(preset.defaultModel); setAPIMode(preset.apiMode);
		patchCurrent({ provider: 'custom', base_url: preset.baseURL, model: preset.defaultModel, api_mode: preset.apiMode }); setModels([]);
	};

	const persist = async () => {
		if (!config) throw new Error('後端尚未連線');
		const llmProfiles = profiles.map((item) => {
			const value = item.id === activeID ? { ...item, name: profileName.trim(), provider, base_url: baseURL.trim(), model: model.trim(), api_mode: apiMode } : item;
			return { id: value.id, name: value.name.trim(), provider: value.provider, base_url: value.base_url.trim(), model: value.model.trim(), api_mode: value.api_mode, api_key: profileKeyValues[value.id]?.trim() || undefined, clear_api_key: clearProfileKeys.has(value.id) };
		});
		const result = await requestJSON<{ data: AppSettings }>(config, '/api/v1/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ llm: { response_timeout_seconds: timeout }, llm_profiles: llmProfiles, active_llm_profile_id: activeID }) });
		const saved = normalizeProfiles(result.data); const selected = saved.find((item) => item.id === result.data.active_llm_profile_id) || saved[0];
		setSettings(result.data); setProfiles(saved); setActiveID(selected.id); loadProfile(selected); setProfileKeyValues({}); setClearProfileKeys(new Set()); onSaved?.(result.data);
		return result.data;
	};
	const save = async () => { setState('saving'); setMessage(''); try { await persist(); setState('saved'); setMessage('模型設定已儲存'); } catch (error) { setState('error'); setMessage(error instanceof Error ? error.message : '儲存模型設定失敗'); } };
	const discover = async () => {
		if (!config) return; setState('saving'); setMessage('');
		try {
			const result = await requestJSON<{ data: { models: LLMModelOption[] } }>(config, '/api/v1/settings/llm/models', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider, base_url: baseURL, profile_id: activeID, api_key: profileKeyValues[activeID]?.trim() || undefined }) });
			setModels(result.data.models); setState('idle'); setMessage(`已取得 ${result.data.models.length} 個模型`);
		} catch (error) { setState('error'); setMessage(llmLocalConnectionError(baseURL, error)); }
	};
	const testConnection = async () => {
		if (!config) return; setState('saving'); setMessage('');
		try { await persist(); const result = await requestJSON<{ data: LLMConnectionTestResult }>(config, '/api/v1/settings/llm/test', { method: 'POST' }); setTestResult(result.data); setState('saved'); setMessage(`連線成功，耗時 ${result.data.latency_ms}ms`); }
		catch (error) { setState('error'); setMessage(llmLocalConnectionError(baseURL, error)); }
	};

	if (!open || !settings) return <section className="settings-section"><div className="settings-section-title"><PlugZap size={18} /><div><h3>AI 模型</h3><p>{state === 'loading' ? '讀取中…' : message || '尚未載入模型設定'}</p></div></div></section>;
	return <section className="settings-section model-settings-panel">
		<div className="settings-section-title"><PlugZap size={18} /><div><h3>AI 模型執行環境</h3><p>模型密鑰只顯示遮罩狀態；留空會保留既有密鑰。</p></div></div>
		<div className={`llm-connection-test ${runtime?.available ? runtime.configured ? 'success' : '' : 'error'}`}><div><PlugZap size={17} /><span><strong>{runtime?.available ? `AI Runtime ${runtime.version || ''}` : 'AI 執行環境不可用'}</strong><small>{runtime?.message || (runtime?.configured ? '執行環境和模型設定均已就緒。' : '執行環境已就緒，請繼續設定模型連線。')}</small></span></div></div>
		<div className="llm-profile-toolbar"><label><span>模型設定</span><select value={activeID} onChange={(event) => selectProfile(event.target.value)}>{profiles.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></label><button type="button" onClick={addProfile}><Plus size={14} />新增設定</button><button type="button" className="danger" onClick={removeProfile} disabled={profiles.length <= 1}><Trash2 size={14} />刪除</button></div>
		<label><span>設定名稱</span><input value={profileName} onChange={(event) => { setProfileName(event.target.value); patchCurrent({ name: event.target.value }); }} /></label>
		<div className="settings-grid two-columns"><label><span>Provider</span><select value={provider} onChange={(event) => changeProvider(event.target.value)}>{llmProviders.map((item) => <option value={item.id} key={item.id}>{item.label}</option>)}</select></label><label><span>API mode</span><select value={apiMode} onChange={(event) => { setAPIMode(event.target.value); patchCurrent({ api_mode: event.target.value }); }}><option value="chat_completions">Chat Completions</option><option value="codex_responses">Responses API</option><option value="anthropic_messages">Anthropic Messages</option></select></label></div>
		<div className="llm-local-presets"><span>本地模型</span><div>{llmLocalPresets.map((item) => <button type="button" key={item.id} onClick={() => applyLocalPreset(item)}>{item.label}</button>)}</div><small>使用 OpenAI-compatible 介面；API Key 可留空。</small></div>
		<label><span>API Base URL</span><input value={baseURL} onChange={(event) => { setBaseURL(event.target.value); patchCurrent({ base_url: event.target.value }); setModels([]); }} /></label>
		<label className={`secret-field ${clearProfileKeys.has(activeID) ? 'clearing' : ''}`}><span>模型 API Key {current?.api_key.configured && <em>已設定 {current.api_key.masked || ''}</em>}</span><input type="password" autoComplete="new-password" disabled={clearProfileKeys.has(activeID)} value={profileKeyValues[activeID] || ''} onChange={(event) => { setProfileKeyValues((values) => ({ ...values, [activeID]: event.target.value })); setClearProfileKeys((values) => { const copy = new Set(values); copy.delete(activeID); return copy; }); }} placeholder={current?.api_key.configured ? '留空保留現有密鑰' : '輸入 API Key'} /><button type="button" onClick={() => { setProfileKeyValues((values) => ({ ...values, [activeID]: '' })); setClearProfileKeys((values) => { const copy = new Set(values); if (copy.has(activeID)) copy.delete(activeID); else copy.add(activeID); return copy; }); }} disabled={!current?.api_key.configured}>{clearProfileKeys.has(activeID) ? '取消清除' : '清除已儲存密鑰'}</button></label>
		<label><span>模型</span><input list="llm-model-options" value={model} onChange={(event) => { setModel(event.target.value); patchCurrent({ model: event.target.value }); }} placeholder="輸入或取得模型清單" /><datalist id="llm-model-options">{models.map((item) => <option value={item.id} key={item.id}>{item.display_name || item.id}</option>)}</datalist></label>
		<label><span>模型回應逾時（秒）</span><input type="number" min={30} max={3600} value={timeout} onChange={(event) => setTimeoutValue(Number(event.target.value) || defaultTimeout)} /></label>
		<div className="settings-footer"><span>{state === 'error' ? <CircleAlert size={14} /> : state === 'saved' ? <CheckCircle2 size={14} /> : null}{message || testResult?.response || ''}</span><button type="button" onClick={() => void discover()} disabled={state === 'saving'}>取得模型清單</button><button type="button" onClick={() => void testConnection()} disabled={state === 'saving'}><PlugZap size={14} />測試連線</button><button type="button" onClick={() => void save()} disabled={state === 'saving'}>{state === 'saving' ? <LoaderCircle className="spin" size={14} /> : <Save size={14} />}儲存</button></div>
	</section>;
}
