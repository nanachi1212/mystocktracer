import { Check, Copy, ExternalLink, X } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { Intelligence } from './TaiwanStockResearchWorkspace';
import {
	SUBSCRIPTION_AI_ANALYSIS_TYPES,
	SUBSCRIPTION_AI_PROVIDERS,
	buildSubscriptionAIPrompt,
	type SubscriptionAIAnalysisType,
	type SubscriptionAIProvider,
} from '../lib/subscription-ai';

export interface SubscriptionAIModalProps {
	intelligence: Intelligence;
	onClose: () => void;
}

export function SubscriptionAIModal({ intelligence, onClose }: SubscriptionAIModalProps) {
	const [selectedProvider, setSelectedProvider] = useState<SubscriptionAIProvider>('chatgpt');
	const [analysisType, setAnalysisType] = useState<SubscriptionAIAnalysisType>('comprehensive');
	const [actionMessage, setActionMessage] = useState<string>('');
	const [actionError, setActionError] = useState<string>('');
	const [busy, setBusy] = useState<boolean>(false);

	const promptText = useMemo(() => {
		return buildSubscriptionAIPrompt(intelligence, analysisType);
	}, [intelligence, analysisType]);

	const providerInfo = useMemo(() => {
		return SUBSCRIPTION_AI_PROVIDERS.find((p) => p.id === selectedProvider) || SUBSCRIPTION_AI_PROVIDERS[0];
	}, [selectedProvider]);

	const handleCopyAndOpen = async () => {
		if (busy) return;
		setBusy(true);
		setActionMessage('');
		setActionError('');

		// Step 1: 建立 prompt 並複製到剪貼簿
		try {
			if (!navigator?.clipboard?.writeText) {
				throw new Error('瀏覽器不支援或未允許剪貼簿寫入權限');
			}
			await navigator.clipboard.writeText(promptText);
		} catch (err: any) {
			setBusy(false);
			setActionError(`剪貼簿複製失敗: ${err?.message || String(err)}。未開啟瀏覽器。`);
			return;
		}

		// Step 2: 剪貼簿成功後，開啟 AI 官方網站
		const targetUrl = providerInfo.url;
		const stockDisplayName = `${intelligence.identity.code} ${intelligence.identity.name}`;

		try {
			if (window.aStock?.openSubscriptionAI) {
				await window.aStock.openSubscriptionAI(targetUrl);
			} else {
				// 備用: 一般網頁環境安全 open
				const win = window.open(targetUrl, '_blank', 'noopener,noreferrer');
				if (!win) {
					throw new Error('視窗被彈出視窗封鎖程式攔截');
				}
			}
			setActionMessage(`已複製 ${stockDisplayName} 的分析資料。\n請在 ${providerInfo.name} 按 Ctrl+V 貼上並送出。`);
		} catch (err: any) {
			setActionMessage(`資料已複製，但無法自動開啟瀏覽器 (${err?.message || String(err)})，請手動開啟官方網站: ${targetUrl}`);
		} finally {
			setBusy(false);
		}
	};

	return (
		<div className="subscription-ai-modal-overlay" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="subscription-ai-title">
			<div className="subscription-ai-modal" onClick={(e) => e.stopPropagation()}>
				<header className="subscription-ai-modal-header">
					<div>
						<span>使用已訂閱的 AI</span>
						<h2 id="subscription-ai-title">
							{intelligence.identity.name} <small>{intelligence.identity.code} · {intelligence.identity.canonical_symbol}</small>
						</h2>
						<p>產出客觀證券研究提示詞並複製至剪貼簿，於外部 AI 官方網頁貼上進行深入對話。</p>
					</div>
					<button type="button" className="subscription-ai-modal-close" onClick={onClose} aria-label="關閉">
						<X size={18} />
					</button>
				</header>

				<div className="subscription-ai-modal-body">
					{/* AI 選擇 */}
					<div className="subscription-ai-section">
						<label className="subscription-ai-section-title">選擇欲前往的外部 AI 服務：</label>
						<div className="subscription-ai-provider-group">
							{SUBSCRIPTION_AI_PROVIDERS.map((provider) => (
								<button
									key={provider.id}
									type="button"
									className={`subscription-ai-provider-btn${selectedProvider === provider.id ? ' active' : ''}`}
									onClick={() => setSelectedProvider(provider.id)}
								>
									{provider.name}
								</button>
							))}
						</div>
					</div>

					{/* 分析方式 */}
					<div className="subscription-ai-section">
						<label className="subscription-ai-section-title">選擇分析方式：</label>
						<div className="subscription-ai-type-options">
							{SUBSCRIPTION_AI_ANALYSIS_TYPES.map((typeOption) => (
								<label key={typeOption.id} className="subscription-ai-type-radio">
									<input
										type="radio"
										name="subscription-ai-analysis-type"
										value={typeOption.id}
										checked={analysisType === typeOption.id}
										onChange={() => setAnalysisType(typeOption.id)}
									/>
									<span className="radio-content">
										<strong>{typeOption.label}</strong>
										<small>{typeOption.description}</small>
									</span>
								</label>
							))}
						</div>
					</div>

					{/* 即將提供給 AI 的資料預覽 */}
					<div className="subscription-ai-section">
						<div className="subscription-ai-preview-header">
							<label className="subscription-ai-section-title">即將提供給 AI 的資料（可滾動預覽完整內容）：</label>
							<span className="subscription-ai-privacy-note">僅包含目前選中證券之公開客觀證據，絕無私密金鑰或投資組合資訊</span>
						</div>
						<div className="subscription-ai-prompt-preview">
							<pre>{promptText}</pre>
						</div>
					</div>

					{/* 狀態訊息 */}
					{actionError && (
						<div className="subscription-ai-error" role="alert">
							{actionError}
						</div>
					)}
					{actionMessage && (
						<div className="subscription-ai-success" role="status">
							<Check size={16} />
							<div className="subscription-ai-success-text">
								{actionMessage.split('\n').map((line, idx) => (
									<div key={idx}>{line}</div>
								))}
							</div>
						</div>
					)}
				</div>

				<footer className="subscription-ai-modal-footer">
					<button type="button" className="subscription-ai-btn-secondary" onClick={onClose}>
						取消
					</button>
					<button
						type="button"
						className="subscription-ai-btn-primary"
						onClick={handleCopyAndOpen}
						disabled={busy}
					>
						<Copy size={15} />
						<ExternalLink size={15} />
						複製並開啟 {providerInfo.name}
					</button>
				</footer>
			</div>
		</div>
	);
}
