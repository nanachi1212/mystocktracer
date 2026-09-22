import { logRuntimeEvent, runtimeErrorDetails, runtimeFeatureForPath } from './runtime-log';

export type BackendConfig = {
  backendUrl: string;
  token: string;
};

export type BackendBridge = {
  getBackendConfig: () => Promise<BackendConfig>;
	getRuntimeLogStatus?: () => Promise<RuntimeLogStatus>;
	openRuntimeLogs?: () => Promise<void>;
	logRuntimeEvent?: (entry: RuntimeLogEntry) => Promise<boolean>;
  getUpdateStatus?: () => Promise<AppUpdateStatus>;
  checkForUpdates?: () => Promise<AppUpdateStatus>;
  downloadUpdate?: () => Promise<AppUpdateStatus>;
  installUpdate?: () => Promise<AppUpdateStatus>;
  openUpdateRelease?: () => Promise<void>;
  openUpdateBackups?: () => Promise<void>;
  onUpdateStatus?: (listener: (status: AppUpdateStatus) => void) => () => void;
  openSubscriptionAI?: (targetUrl: string) => Promise<void>;
};

export type RuntimeLogStatus = {
	available: boolean;
	directory: string;
	max_file_mb: number;
	backup_files: number;
};

export type RuntimeLogEntry = {
	level: 'debug' | 'info' | 'warn' | 'error';
	feature: string;
	message: string;
};

export type AppUpdateState = 'disabled' | 'idle' | 'checking' | 'available' | 'downloading' | 'downloaded' | 'not-available' | 'error' | 'installing';

export type AppUpdateStatus = {
	state: AppUpdateState;
	supported: boolean;
	installMode?: 'automatic' | 'manual';
	currentVersion: string;
	latestVersion?: string;
	releaseName?: string;
	releaseNotes?: string;
	message: string;
	progress: number;
	transferred?: number;
	total?: number;
	bytesPerSecond?: number;
	backupPath?: string;
	backupCreatedAt?: string;
};

export type SecretSettingStatus = {
	configured: boolean;
	masked?: string;
};

export type AppSettings = {
	agent?: {
		available: boolean;
		configured: boolean;
		api_key_configured: boolean;
		version?: string;
		message?: string;
	};
	hermes: {
		available: boolean;
		configured: boolean;
		api_key_configured: boolean;
		version?: string;
		message?: string;
	};
	llm: {
		provider: string;
		base_url: string;
		model: string;
		api_mode: 'chat_completions' | 'responses' | 'anthropic_messages' | string;
		response_timeout_seconds: number;
		api_key: SecretSettingStatus;
	};
	llm_profiles: LLMProfile[];
	active_llm_profile_id: string;
	taiwan_alerts: {
		corporate_events_enabled: boolean;
	};
	updated_at?: string;
};

export type LLMProfile = {
	id: string;
	name: string;
	provider: string;
	base_url: string;
	model: string;
	api_mode: 'chat_completions' | 'responses' | 'codex_responses' | 'anthropic_messages' | string;
	api_key: SecretSettingStatus;
};

export type AgentSkillSetting = {
	name: string;
	description: string;
	category: string;
	enabled: boolean;
};

export type AgentMCPServerSetting = {
	name: string;
	enabled: boolean;
	transport: 'stdio' | 'http' | 'sse';
	command?: string;
	args?: string[];
	env?: Record<string, SecretSettingStatus>;
	url?: string;
	headers?: Record<string, SecretSettingStatus>;
	timeout?: number;
	connect_timeout?: number;
	supports_parallel_tool_calls?: boolean;
};

export type AgentSettings = {
  reasoning_effort: 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max' | string;
  skills: AgentSkillSetting[];
  mcp_servers: AgentMCPServerSetting[];
};


export type LLMConnectionTestResult = {
	ok: boolean;
	provider: string;
	model: string;
	api_mode: string;
	runtime: 'hermes' | string;
	latency_ms: number;
	response: string;
};

export type LLMModelOption = {
	id: string;
	owned_by?: string;
	display_name?: string;
};

export type LLMModelsResult = {
	models: LLMModelOption[];
	source_url: string;
};

export type ResolveInput = {
  bridge?: BackendBridge;
  env?: Record<string, string | undefined>;
};

export class BackendRequestError extends Error {
	readonly status: number;
	readonly requestID: string;
	readonly code?: string;

	constructor(message: string, options: { status?: number; requestID?: string; code?: string } = {}) {
		super(message);
		this.name = 'BackendRequestError';
		this.status = options.status ?? 0;
		this.requestID = options.requestID ?? '';
		this.code = options.code;
	}
}

export type SourceMeta = {
  source: string;
  source_url?: string;
  available_fields?: string[];
  fetched_at: string;
  latency_ms: number;
  stale: boolean;
  trade_date?: string;
  snapshot_id?: string;
  next_refresh_at?: string;
  fallback_reason?: string;
	carry_forward?: boolean;
	status?: string;
	is_realtime?: boolean;
};

export type SecurityIdentity = {
	canonical: string;
	code: string;
	name: string;
	full_name?: string;
	market: string;
	exchange: 'TWSE' | 'TPEX' | string;
	security_type: 'stock' | 'etf' | 'index' | string;
	currency: string;
	timezone: string;
	provider: string;
	source_url: string;
	retrieved_at: string;
};

export type InstitutionalFlow = { canonical:string; trade_date:string; unit:string; foreign_buy:number; foreign_sell:number; foreign_net:number; investment_trust_buy:number; investment_trust_sell:number; investment_trust_net:number; dealer_buy:number; dealer_sell:number; dealer_net:number; meta:SourceMeta };
export type InstitutionalHistory = { security:SecurityIdentity; data:InstitutionalFlow[]; summary:{ foreign_net_5d:number; foreign_net_20d:number; investment_trust_net_5d:number; investment_trust_net_20d:number; dealer_net_5d:number; dealer_net_20d:number; foreign_consecutive_buy_days:number; foreign_consecutive_sell_days:number; investment_trust_consecutive_buy_days:number; investment_trust_consecutive_sell_days:number }; meta:SourceMeta };
export type MarginTrading = { canonical:string; trade_date:string; unit:string; margin_balance?:number; margin_buy?:number; margin_sell?:number; margin_change?:number; short_balance?:number; short_sell?:number; short_cover?:number; short_change?:number; short_margin_ratio?:number; meta:SourceMeta };
export type MarginHistory = { security:SecurityIdentity; data:MarginTrading[]; meta:SourceMeta };
export type FundamentalCapability = { status:string; provider?:string; source?:string; source_url?:string; retrieved_at:string; reason?:string };
export type MonthlyRevenue = { canonical:string; period:string; revenue:number; computed_mom_percent?:number; computed_yoy_percent?:number; provider:string; source:string; source_url:string; status:string };
export type FinancialStatementPeriod = { fiscal_year:number; fiscal_quarter:number; period_end:string; revenue?:number; gross_profit?:number; operating_income?:number; net_income_attributable_to_parent?:number; cumulative_eps?:number; published_at?:string; available_at?:string; provider:string; status:string };
export type ValuationSnapshot = { data_date:string; pe?:number; pb?:number; dividend_yield_percent?:number; provider:string; status:string };
export type DividendRecord = { year:number; cash_dividend?:number; stock_dividend?:number; ex_dividend_date?:string; raw_status?:string; normalized_status:string; provider:string; status:string };
export type TaiwanFundamentals = { security:SecurityIdentity; monthly_revenue:MonthlyRevenue[] | null; financial_statement?:FinancialStatementPeriod; valuation?:ValuationSnapshot; dividends:DividendRecord[] | null; capabilities:Record<string,FundamentalCapability>; meta:SourceMeta };

export type Quote = {
  symbol: string;
  name: string;
  price: number;
  open: number;
  previous_close: number;
  high: number;
  low: number;
  change: number;
  change_percent: number;
  trade_time?: string;
  meta: SourceMeta;
};

export type KLine = {
  symbol: string;
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  previous_close?: number;
  volume: number;
  amount: number;
  turnover_rate?: number;
  change_percent?: number;
  meta: SourceMeta;
};

export type MarketIndexSnapshot = {
	id: string;
	secid: string;
	code: string;
	name: string;
	region: string;
	market: string;
	currency: string;
	price: number;
	change: number;
	change_percent: number;
	trade_time?: string;
	status: string;
	meta: SourceMeta;
};

export type MarketIndexSeries = {
	index: MarketIndexSnapshot;
	lines: KLine[];
	meta: SourceMeta;
};

export async function resolveBackendConfig(input: ResolveInput = {}): Promise<BackendConfig> {
  const bridge = input.bridge ?? globalThis.window?.aStock;
  const bridged = await bridge?.getBackendConfig().catch(() => undefined);
  if (bridged?.backendUrl) {
    return normalizeConfig(bridged);
  }

  const env = input.env ?? import.meta.env;
  return normalizeConfig({
    backendUrl: env.VITE_MYSTOCKTRACER_BACKEND_URL ?? env.VITE_A_STOCK_BACKEND_URL ?? 'http://127.0.0.1:20081',
    token: env.VITE_MYSTOCKTRACER_TOKEN ?? env.VITE_A_STOCK_TOKEN ?? '',
  });
}

export async function requestJSON<T>(config: BackendConfig, path: string, init: RequestInit = {}): Promise<T> {
	const safeConfig = normalizeConfig(config);
	const headers = new Headers(init.headers);
	if (safeConfig.token) headers.set('Authorization', 'Bearer ' + safeConfig.token);
	const requestID = createRuntimeRequestID();
	headers.set('X-Request-ID', requestID);
	const requestURL = new URL(path, safeConfig.backendUrl);
	const backendOrigin = new URL(safeConfig.backendUrl).origin;
	if (requestURL.origin !== backendOrigin) {
		throw new BackendRequestError('後端請求不得離開已設定的本機來源', { requestID, code: 'cross_origin_backend_request' });
	}
	let response: Response;
	try {
		response = await fetch(requestURL, { ...init, headers });
	} catch (error) {
		logRuntimeEvent('error', runtimeFeatureForPath(requestURL.pathname), {
			event: 'network_failure', request_id: requestID, method: init.method || 'GET', path: requestURL.pathname, error: runtimeErrorDetails(error),
		});
		if (error instanceof DOMException && error.name === 'AbortError') throw error;
		throw new BackendRequestError(error instanceof Error ? error.message : '無法連線至本機後端', {
			requestID,
			code: 'network_error',
		});
	}
	const responseRequestID = response.headers.get('X-Request-ID') || requestID;
  if (!response.ok) {
		logRuntimeEvent('warn', runtimeFeatureForPath(requestURL.pathname), {
			event: 'http_failure', request_id: responseRequestID, method: init.method || 'GET', path: requestURL.pathname, status: response.status,
		});
    const text = await response.text();
		let message = text.trim();
		let code: string | undefined;
		try {
			const payload = JSON.parse(text) as { error?: string; code?: string };
			message = payload.error || message;
			code = payload.code;
		} catch {
			// A local proxy can still return plain text; preserve it as the diagnostic message.
		}
		throw new BackendRequestError(message || 'HTTP ' + response.status, {
			status: response.status,
			requestID: responseRequestID,
			code,
		});
  }
	if (response.status === 204 || response.status === 205) return undefined as T;
	const text = await response.text();
	if (text.trim() === '') return undefined as T;
	try {
		return JSON.parse(text) as T;
	} catch {
		throw new BackendRequestError('後端回應不是有效的 JSON', {
			status: response.status,
			requestID: responseRequestID,
			code: 'invalid_json',
		});
	}
}

function createRuntimeRequestID() {
	if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
	return `renderer-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

function normalizeConfig(config: BackendConfig): BackendConfig {
	let url: URL;
	try {
		url = new URL(config.backendUrl);
	} catch {
		throw new BackendRequestError('後端網址無效', { code: 'invalid_backend_url' });
	}
	const hostname = url.hostname.toLowerCase();
	if ((url.protocol !== 'http:' && url.protocol !== 'https:') || !['localhost', '127.0.0.1', '::1'].includes(hostname)) {
		throw new BackendRequestError('後端網址必須是使用 HTTP 或 HTTPS 的本機 loopback 位址', { code: 'invalid_backend_url' });
	}
  return {
    backendUrl: url.toString().replace(/\/+$/, ''),
    token: config.token || '',
  };
}
