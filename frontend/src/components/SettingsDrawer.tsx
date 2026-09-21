import {
	Bot,
	Bell,
	CheckCircle2,
	CircleAlert,
	Eye,
	EyeOff,
	FolderOpen,
	KeyRound,
	LoaderCircle,
	PlugZap,
	Plus,
	RefreshCw,
	Save,
	ShieldCheck,
	Trash2,
	X,
} from 'lucide-react';
import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { AppSettings, BackendConfig, LLMConnectionTestResult, LLMModelOption, LLMModelsResult, LLMProfile, RuntimeLogStatus, SecretSettingStatus, requestJSON } from '../lib/backend';
import { llmLocalConnectionError, llmLocalPresets, llmProviderDefinition, llmProviders } from '../lib/llm-providers';
import { AppUpdatePanel } from './AppUpdatePanel';
import { HermesAgentSettingsPanel } from './HermesAgentSettingsPanel';

type Props = {
	config: BackendConfig | null;
	open: boolean;
	onClose: () => void;
	onSaved?: () => void;
};

// Phase B1 removed the China data-provider credentials and review-source automation, so the only
// secret this drawer still manages is the Hermes model API key.
type SecretKey = 'llm_api_key';
type ModelListState = 'idle' | 'loading' | 'success' | 'error';

const manualModelOption = '__manual_model_input__';
const defaultResponseTimeoutSeconds = 300;


export function SettingsDrawer({ config, open, onClose, onSaved }: Props) {
	const [settings, setSettings] = useState<AppSettings | null>(null);
	const [llmProfiles, setLLMProfiles] = useState<LLMProfile[]>([]);
	const [activeLLMProfileID, setActiveLLMProfileID] = useState('');
	const [profileName, setProfileName] = useState('');
	const [profileKeyValues, setProfileKeyValues] = useState<Record<string, string>>({});
	const [clearProfileKeys, setClearProfileKeys] = useState<Set<string>>(new Set());
	const [provider, setProvider] = useState('openai');
	const [baseURL, setBaseURL] = useState(llmProviderDefinition('openai').baseURL);
	const [model, setModel] = useState('');
	const [apiMode, setAPIMode] = useState('chat_completions');
	const [responseTimeoutSeconds, setResponseTimeoutSeconds] = useState(defaultResponseTimeoutSeconds);
	const [corporateEventAlertsEnabled, setCorporateEventAlertsEnabled] = useState(true);
	const [state, setState] = useState<'idle' | 'loading' | 'saving' | 'saved' | 'error'>('idle');
	const [message, setMessage] = useState('');
	const [testState, setTestState] = useState<'idle' | 'testing' | 'success' | 'error'>('idle');
	const [testResult, setTestResult] = useState<LLMConnectionTestResult | null>(null);
	const [modelOptions, setModelOptions] = useState<LLMModelOption[]>([]);
	const [modelListState, setModelListState] = useState<ModelListState>('idle');
	const [modelListMessage, setModelListMessage] = useState('');
	const [manualModel, setManualModel] = useState(true);
	const [runtimeLogStatus, setRuntimeLogStatus] = useState<RuntimeLogStatus | null>(null);
	const [openingRuntimeLogs, setOpeningRuntimeLogs] = useState(false);
	const modelFetchSequence = useRef(0);

	useEffect(() => {
		if (!open) return;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Escape') onClose();
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [onClose, open]);

	useEffect(() => {
		if (!open || !config) return;
		let cancelled = false;
		modelFetchSequence.current += 1;
		setState('loading');
		setMessage('');
		setTestState('idle');
		setTestResult(null);
		setModelOptions([]);
		setModelListState('idle');
		setModelListMessage('');
		setManualModel(true);
		requestJSON<{ data: AppSettings }>(config, '/api/v1/settings')
			.then((payload) => {
				if (cancelled) return;
				setSettings(payload.data);
				const modelProfiles = normalizeLLMProfiles(payload.data);
				setLLMProfiles(modelProfiles);
				const selected = modelProfiles.find((profile) => profile.id === payload.data.active_llm_profile_id) || modelProfiles[0];
				setActiveLLMProfileID(selected.id);
				loadProfileFields(selected);
				setResponseTimeoutSeconds(payload.data.llm.response_timeout_seconds || defaultResponseTimeoutSeconds);
				setProfileKeyValues({});
				setClearProfileKeys(new Set());
				setCorporateEventAlertsEnabled(payload.data.taiwan_alerts?.corporate_events_enabled !== false);
				setState('idle');
			})
			.catch((error) => {
				if (cancelled) return;
				setState('error');
				setMessage(error instanceof Error ? error.message : '讀取設定失敗');
			});
		return () => {
			cancelled = true;
			modelFetchSequence.current += 1;
		};
	}, [config, open]);

	useEffect(() => {
		if (!open || !window.aStock?.getRuntimeLogStatus) {
			setRuntimeLogStatus(null);
			return;
		}
		void window.aStock.getRuntimeLogStatus().then(setRuntimeLogStatus).catch(() => setRuntimeLogStatus(null));
	}, [open]);

	const configuredCount = useMemo(
		() => settings ? settings.llm_profiles.filter((profile) => profile.api_key.configured).length : 0,
		[settings],
	);

	const selectedLLMProfile = useMemo(() => llmProfiles.find((profile) => profile.id === activeLLMProfileID), [activeLLMProfileID, llmProfiles]);

	const patchSelectedLLMProfile = (patch: Partial<LLMProfile>) => setLLMProfiles((current) => current.map((profile) => profile.id === activeLLMProfileID ? { ...profile, ...patch } : profile));

	const loadProfileFields = (profile: LLMProfile) => {
		setProfileName(profile.name);
		setProvider(profile.provider || 'openai');
		setBaseURL(profile.base_url || llmProviderDefinition(profile.provider || 'openai').baseURL);
		setModel(profile.model || '');
		setAPIMode(profile.api_mode === 'responses' ? 'codex_responses' : profile.api_mode || (profile.provider === 'anthropic' ? 'anthropic_messages' : 'chat_completions'));
	};

	const selectLLMProfile = (id: string) => {
		const profile = llmProfiles.find((item) => item.id === id);
		if (!profile) return;
		setActiveLLMProfileID(id);
		loadProfileFields(profile);
		resetModelList();
		setTestState('idle');
		setTestResult(null);
	};

	const addLLMProfile = () => {
		const id = `llm-${Date.now()}-${Math.random().toString(16).slice(2)}`;
		const definition = llmProviderDefinition('openai');
		const profile: LLMProfile = { id, name: '新模型配置', provider: 'openai', base_url: definition.baseURL, model: definition.defaultModel, api_mode: definition.apiMode, api_key: { configured: false } };
		setLLMProfiles((current) => [...current, profile]);
		setActiveLLMProfileID(id);
		loadProfileFields(profile);
		resetModelList();
	};

	const removeLLMProfile = () => {
		if (llmProfiles.length <= 1) return;
		const remaining = llmProfiles.filter((profile) => profile.id !== activeLLMProfileID);
		const next = remaining[0];
		setLLMProfiles(remaining);
		setActiveLLMProfileID(next.id);
		loadProfileFields(next);
		setProfileKeyValues((current) => { const copy = { ...current }; delete copy[activeLLMProfileID]; return copy; });
		setClearProfileKeys((current) => { const copy = new Set(current); copy.delete(activeLLMProfileID); return copy; });
		resetModelList();
	};

	const selectableModels = useMemo(() => {
		const currentModel = model.trim();
		if (!currentModel || modelOptions.some((option) => option.id === currentModel)) return modelOptions;
		return [{ id: currentModel, display_name: '当前配置' }, ...modelOptions];
	}, [model, modelOptions]);

	const resetModelList = () => {
		modelFetchSequence.current += 1;
		setModelOptions([]);
		setModelListState('idle');
		setModelListMessage('');
		setManualModel(true);
	};

	const updateProvider = (nextProvider: string) => {
		const nextDefinition = llmProviderDefinition(nextProvider);
		setProvider(nextProvider);
		setBaseURL(nextDefinition.baseURL);
		setModel(nextDefinition.defaultModel);
		setAPIMode(nextDefinition.apiMode);
		patchSelectedLLMProfile({ provider: nextProvider, base_url: nextDefinition.baseURL, model: nextDefinition.defaultModel, api_mode: nextDefinition.apiMode });
		resetModelList();
		setTestState('idle');
		setTestResult(null);
		setModelListMessage(nextDefinition.baseURL
			? '請確認 Base URL，輸入模型 API Key，然後點擊「取得模型清單」。'
			: '請輸入相容介面的 Base URL 和 API Key，再取得模型清單。');
	};

	const applyLocalPreset = (preset: typeof llmLocalPresets[number]) => {
		setProvider('custom'); setBaseURL(preset.baseURL); setModel(preset.defaultModel); setAPIMode(preset.apiMode);
		patchSelectedLLMProfile({ provider: 'custom', base_url: preset.baseURL, model: preset.defaultModel, api_mode: preset.apiMode });
		resetModelList(); setTestState('idle'); setTestResult(null);
		setModelListMessage(`${preset.label} 使用既有 OpenAI-compatible 介面；API Key 可留空。`);
	};

	const updateSecret = (_key: SecretKey, value: string) => {
		setProfileKeyValues((current) => ({ ...current, [activeLLMProfileID]: value }));
		setClearProfileKeys((current) => { const next = new Set(current); if (value) next.delete(activeLLMProfileID); return next; });
		resetModelList(); setTestState('idle'); setTestResult(null);
	};

	const toggleClear = (_key: SecretKey) => {
		setClearProfileKeys((current) => { const next = new Set(current); if (next.has(activeLLMProfileID)) next.delete(activeLLMProfileID); else next.add(activeLLMProfileID); return next; });
		setProfileKeyValues((current) => ({ ...current, [activeLLMProfileID]: '' }));
		resetModelList();
	};

	const updateModel = (nextModel: string) => {
		setModel(nextModel);
		patchSelectedLLMProfile({ model: nextModel });
		setTestState('idle');
		setTestResult(null);
	};

	const updateBaseURL = (nextBaseURL: string) => {
		setBaseURL(nextBaseURL);
		patchSelectedLLMProfile({ base_url: nextBaseURL });
		resetModelList();
		setTestState('idle');
		setTestResult(null);
	};

	const fetchModels = async () => {
		if (!config) return;
		const fetchID = ++modelFetchSequence.current;
		const requestProvider = provider;
		const requestBaseURL = baseURL.trim();
		const requestAPIKey = (profileKeyValues[activeLLMProfileID] || '').trim();
		setModelListState('loading');
		setModelListMessage('正在讀取模型服務的模型清單…');
		try {
			const request: { provider: string; base_url: string; api_key?: string } = { provider: requestProvider, base_url: requestBaseURL };
			if (requestAPIKey || clearProfileKeys.has(activeLLMProfileID)) request.api_key = requestAPIKey;
			(request as { profile_id?: string }).profile_id = activeLLMProfileID;
			const payload = await requestJSON<{ data: LLMModelsResult }>(config, '/api/v1/settings/llm/models', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(request),
			});
			if (fetchID !== modelFetchSequence.current) return;
			setModelOptions(payload.data.models);
			setModelListState('success');
			setModelListMessage(`已從 ${payload.data.source_url} 取得 ${payload.data.models.length} 個模型`);
			setManualModel(false);
		} catch (error) {
			if (fetchID !== modelFetchSequence.current) return;
			setModelOptions([]);
			setModelListState('error');
			setModelListMessage(llmLocalConnectionError(requestBaseURL, error));
			setManualModel(true);
		}
	};


	const persistSettings = async () => {
		if (!config) throw new Error('後端尚未連線');
		const modelProfiles = llmProfiles.map((profile) => {
			const current = profile.id === activeLLMProfileID ? { ...profile, name: profileName.trim(), provider, base_url: baseURL.trim(), model: model.trim(), api_mode: apiMode } : profile;
			return { id: current.id, name: current.name.trim(), provider: current.provider, base_url: current.base_url.trim(), model: current.model.trim(), api_mode: current.api_mode, api_key: (profileKeyValues[current.id] || '').trim() || undefined, clear_api_key: clearProfileKeys.has(current.id) };
		});
		const payload = await requestJSON<{ data: AppSettings }>(config, '/api/v1/settings', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ llm: { response_timeout_seconds: responseTimeoutSeconds }, llm_profiles: modelProfiles, active_llm_profile_id: activeLLMProfileID, taiwan_alerts: { corporate_events_enabled: corporateEventAlertsEnabled } }),
		});
		setSettings(payload.data);
		const savedProfiles = normalizeLLMProfiles(payload.data);
		setLLMProfiles(savedProfiles);
		const savedActive = savedProfiles.find((profile) => profile.id === payload.data.active_llm_profile_id) || savedProfiles[0];
		setActiveLLMProfileID(savedActive.id);
		loadProfileFields(savedActive);
		setProfileKeyValues({});
		setClearProfileKeys(new Set());
		setCorporateEventAlertsEnabled(payload.data.taiwan_alerts?.corporate_events_enabled !== false);
		return payload.data;
	};

	const save = async (event: FormEvent) => {
		event.preventDefault();
		if (!config) return;
		setState('saving');
		setMessage('');
		try {
			await persistSettings();
			setState('saved');
			setMessage('設定已同步到本機 Hermes');
			onSaved?.();
		} catch (error) {
			setState('error');
			setMessage(error instanceof Error ? error.message : '儲存設定失敗');
		}
	};

	const testConnection = async () => {
		if (!config) return;
		setState('saving');
		setMessage('正在儲存設定並透過 Hermes 呼叫模型探測');
		setTestState('testing');
		setTestResult(null);
		try {
			await persistSettings();
			const payload = await requestJSON<{ data: LLMConnectionTestResult }>(config, '/api/v1/settings/llm/test', { method: 'POST' });
			setTestResult(payload.data);
			setTestState('success');
			setState('saved');
			setMessage(`Hermes 模型連線成功，耗時 ${payload.data.latency_ms}ms`);
		} catch (error) {
			setTestState('error');
			setState('error');
			setMessage(llmLocalConnectionError(baseURL, error));
		}
	};

	const openRuntimeLogs = async () => {
		if (!window.aStock?.openRuntimeLogs) return;
		setOpeningRuntimeLogs(true);
		try {
			await window.aStock.openRuntimeLogs();
		} catch (error) {
			setState('error');
			setMessage(error instanceof Error ? error.message : '開啟日誌目錄失敗');
		} finally {
			setOpeningRuntimeLogs(false);
		}
	};

	if (!open) return null;

	return (
		<div className="settings-overlay" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
			<aside className="settings-drawer" role="dialog" aria-modal="true" aria-label="系統設定">
				<header className="settings-header">
					<div><span>HERMES LOCAL RUNTIME</span><h2>系統設定</h2><p>管理 Hermes 模型執行環境與 AI 研究所需的連線設定</p></div>
					<button type="button" onClick={onClose} aria-label="關閉設定"><X size={20} /></button>
				</header>

				{state === 'loading' && <div className="settings-loading"><LoaderCircle className="spin" size={22} /><span>讀取本機設定</span></div>}
				{state !== 'loading' && (
					<form className="settings-form" onSubmit={save}>
						<section className="settings-security-note">
							<ShieldCheck size={19} />
							<div><strong>模型密鑰由 Hermes 管理</strong><span>API Key 只寫入 Hermes 的本機 .env；頁面僅讀取是否已設定，不會取回密鑰原文。</span></div>
							<em>{configuredCount} 項已設定</em>
						</section>

						<HermesAgentSettingsPanel config={config} open={open} />

						<section className="settings-section">
							<div className="settings-section-title"><Bot size={18} /><div><h3>Hermes 模型執行環境</h3><p>台股個股分析的「產生 AI 研究摘要」與 AI 助手對話統一由本機 Hermes 驅動。</p></div></div>
							<div className={`llm-connection-test ${settings?.hermes.available ? settings.hermes.configured ? 'success' : '' : 'error'}`}>
								<div><Bot size={17} /><span><strong>{settings?.hermes.available ? `Hermes ${settings.hermes.version || 'Runtime'} 已安裝` : 'Hermes 執行環境不可用'}</strong><small>{settings?.hermes.message || (settings?.hermes.configured ? '執行環境和模型設定均已就緒。' : '執行環境已就緒，請繼續設定模型連線。')}</small></span></div>
							</div>
							<div className="llm-profile-toolbar">
								<label><span>模型設定</span><select value={activeLLMProfileID} onChange={(event) => selectLLMProfile(event.target.value)}>{llmProfiles.map((profile) => <option value={profile.id} key={profile.id}>{profile.name} · {profile.model || llmProviderDefinition(profile.provider).label}</option>)}</select></label>
								<button type="button" onClick={addLLMProfile}><Plus size={14} />新增設定</button>
								<button type="button" className="danger" onClick={removeLLMProfile} disabled={llmProfiles.length <= 1}><Trash2 size={14} />刪除</button>
							</div>
							<label><span>設定名稱</span><input value={profileName} onChange={(event) => { setProfileName(event.target.value); patchSelectedLLMProfile({ name: event.target.value }); }} placeholder="例如 DeepSeek 日常 / GPT-5.6 Sol 深度分析" /></label>
							<div className="settings-grid two-columns">
								<label><span>雲端 API</span><select value={provider} onChange={(event) => updateProvider(event.target.value)}>{llmProviders.map((item) => <option value={item.id} key={item.id}>{item.label}</option>)}</select></label>
								<label><span>介面協定</span><select value={apiMode} onChange={(event) => { setAPIMode(event.target.value); patchSelectedLLMProfile({ api_mode: event.target.value }); setTestState('idle'); setTestResult(null); }}><option value="chat_completions">Chat Completions</option><option value="codex_responses">Responses API</option><option value="anthropic_messages">Anthropic Messages</option></select></label>
							</div>
							<div className="llm-local-presets"><span>本地模型</span><div>{llmLocalPresets.map((preset) => <button type="button" key={preset.id} onClick={() => applyLocalPreset(preset)}>{preset.label}<small>{preset.baseURL}</small></button>)}</div><small>使用既有 OpenAI-compatible 設定；API Key 可留空，Base URL 與 model 都可自行修改。</small></div>
								<label><span>API Base URL</span><input value={baseURL} onChange={(event) => updateBaseURL(event.target.value)} placeholder="https://api.example.com/v1" /></label>
								<SecretField key={`llm-api-key-${activeLLMProfileID}`} label="模型 API Key" secretKey="llm_api_key" status={selectedLLMProfile?.api_key} value={profileKeyValues[activeLLMProfileID] || ''} clearing={clearProfileKeys.has(activeLLMProfileID)} onChange={updateSecret} onClear={toggleClear} hint="每套設定獨立安全保存；切換設定不會覆蓋其他密鑰" revealable />
								<div className="model-field">
									<span className="model-field-heading"><span>模型</span>{modelListState === 'success' && <button type="button" onClick={() => setManualModel((current) => !current)}>{manualModel ? '使用下拉選單' : '手動輸入'}</button>}</span>
									<span className="model-picker-row">
										{modelListState === 'success' && !manualModel ? <select value={model} onChange={(event) => { if (event.target.value === manualModelOption) setManualModel(true); else updateModel(event.target.value); }}><option value="">請選擇模型</option>{selectableModels.map((option) => <option value={option.id} key={option.id}>{modelOptionLabel(option)}</option>)}<option value={manualModelOption}>手動輸入其他模型…</option></select> : <input value={model} onChange={(event) => updateModel(event.target.value)} placeholder="例如 gpt-5.5 或 deepseek-chat" />}
										<button type="button" className="model-refresh-button" onClick={() => void fetchModels()} disabled={!config || state === 'saving' || modelListState === 'loading'}>{modelListState === 'loading' ? <LoaderCircle className="spin" size={14} /> : <RefreshCw size={14} />}{modelListState === 'success' ? '重新整理' : '取得模型清單'}</button>
									</span>
									<small className={`model-list-message ${modelListState}`}>{modelListMessage || '請先填寫 Base URL 和 API Key，再取得模型清單；也可以繼續手動輸入。'}</small>
								</div>
							<label className="settings-timeout-field"><span>模型回應等待時間（秒）</span><input type="number" min={30} max={3600} step={30} value={responseTimeoutSeconds} onChange={(event) => setResponseTimeoutSeconds(Number(event.target.value) || defaultResponseTimeoutSeconds)} /><small>預設 300 秒，範圍 30–3600 秒；模型長時間無回應時會等待更久再重試。</small></label>
							<div className={`llm-connection-test ${testState}`}>
								<div><PlugZap size={17} /><span><strong>{testState === 'success' ? 'Hermes 模型連線可用' : testState === 'error' ? '連線測試未通過' : testState === 'testing' ? 'Hermes 正在請求模型' : 'Hermes 真實模型探測'}</strong><small>{testResult ? `Hermes · ${testResult.model} · ${testResult.api_mode} · ${testResult.latency_ms}ms · ${testResult.response}` : '儲存目前設定後由 Hermes 送出最小提示詞，並驗證模型確實回傳內容。'}</small></span></div>
								<button type="button" onClick={testConnection} disabled={!config || state === 'saving' || testState === 'testing'}>{testState === 'testing' ? <LoaderCircle className="spin" size={15} /> : <PlugZap size={15} />}儲存並測試連線</button>
							</div>
						</section>


						<section className="settings-section">
							<div className="settings-section-title"><Bell size={18} /><div><h3>台股事件提醒</h3><p>控制新公司公告是否加入本機提醒中心；關閉後仍可查看公告，也不會刪除既有提醒。</p></div></div>
							<label className="settings-toggle"><input type="checkbox" checked={corporateEventAlertsEnabled} onChange={(event) => setCorporateEventAlertsEnabled(event.target.checked)} /><span>建立新的公司事件提醒</span></label>
						</section>

						<AppUpdatePanel />

						<section className="settings-section runtime-log-section">
							<div className="settings-section-title"><FolderOpen size={18} /><div><h3>執行日誌</h3><p>遇到問題時，可將此目錄中的日誌檔案提供給開發者排查。</p></div></div>
							<div className="runtime-log-summary">
								<span><strong>{runtimeLogStatus?.available ? '日誌正在自動儲存' : '請在桌面應用程式中查看日誌'}</strong><small>{runtimeLogStatus ? `每個檔案最多 ${runtimeLogStatus.max_file_mb} MB，保留 ${runtimeLogStatus.backup_files} 份歷史記錄；密鑰和登入憑證會在寫入前隱藏。` : '瀏覽器開發模式不會儲存桌面執行日誌。'}</small>{runtimeLogStatus?.directory && <code title={runtimeLogStatus.directory}>{runtimeLogStatus.directory}</code>}</span>
								<button type="button" onClick={() => void openRuntimeLogs()} disabled={!runtimeLogStatus?.available || openingRuntimeLogs}>{openingRuntimeLogs ? <LoaderCircle className="spin" size={15} /> : <FolderOpen size={15} />}開啟日誌目錄</button>
							</div>
						</section>

						<footer className="settings-footer">
							<div className={`settings-message ${state}`}>{state === 'saved' && <CheckCircle2 size={15} />}{state === 'error' && <KeyRound size={15} />}<span>{message || '留空的模型密鑰會保留 Hermes .env 中的現有值。'}</span></div>
							<button type="button" onClick={onClose}>取消</button>
							<button type="submit" className="settings-save" disabled={!config || state === 'saving' || testState === 'testing'}>{state === 'saving' ? <LoaderCircle className="spin" size={16} /> : <Save size={16} />}儲存設定</button>
						</footer>
					</form>
				)}
			</aside>
		</div>
	);
}

function normalizeLLMProfiles(settings: AppSettings): LLMProfile[] {
	if (settings.llm_profiles?.length) return settings.llm_profiles;
	return [{ id: 'llm-default', name: settings.llm.model || llmProviderDefinition(settings.llm.provider).label, provider: settings.llm.provider, base_url: settings.llm.base_url, model: settings.llm.model, api_mode: settings.llm.api_mode, api_key: settings.llm.api_key }];
}

function modelOptionLabel(option: LLMModelOption) {
	const detail = option.display_name || option.owned_by;
	return detail && detail !== option.id ? `${option.id} · ${detail}` : option.id;
}

function SecretField({ label, secretKey, status, value, clearing, onChange, onClear, hint, revealable = false }: {
	label: string;
	secretKey: SecretKey;
	status?: SecretSettingStatus;
	value: string;
	clearing: boolean;
	onChange: (key: SecretKey, value: string) => void;
	onClear: (key: SecretKey) => void;
	hint?: string;
	revealable?: boolean;
}) {
	const [revealed, setRevealed] = useState(false);
	return (
		<label className={`secret-field ${clearing ? 'clearing' : ''}`}>
			<span className="secret-field-heading"><span>{label}{hint && <small>{hint}</small>}</span>{status?.configured && <em>{clearing ? '等待清除' : `已設定 ${status.masked || ''}`}</em>}</span>
			<span className="secret-input-row">
				<KeyRound size={15} />
				<input type={revealable && revealed ? 'text' : 'password'} autoComplete="new-password" value={value} disabled={clearing} onChange={(event) => onChange(secretKey, event.target.value)} placeholder={status?.configured ? '輸入新值可覆蓋，留空保持不變' : '輸入憑證'} />
				{revealable && <button type="button" className="secret-visibility-button" onClick={() => setRevealed((current) => !current)} disabled={clearing || !value} title={revealed ? '隱藏 API Key' : '顯示 API Key'} aria-label={revealed ? '隱藏 API Key' : '顯示 API Key'} aria-pressed={revealed}>{revealed ? <EyeOff size={15} /> : <Eye size={15} />}</button>}
				{status?.configured && <button type="button" className="secret-clear-button" onClick={() => onClear(secretKey)} title={clearing ? '取消清除' : '清除已儲存憑證'}><Trash2 size={14} />{clearing ? '撤銷' : '清除'}</button>}
			</span>
		</label>
	);
}
