import fs from 'node:fs';
import path from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';
import { requestJSON, type BackendConfig } from '../lib/backend';
import { fetchResearchComparison, fetchResearchHistoryPage, fetchResearchHistoryRecord, LatestRequestGate, ResearchComparisonView, ResearchHistoryList, type ResearchComparison, type ResearchHistorySummary } from './TaiwanResearchHistory';

vi.mock('../lib/backend', () => ({ requestJSON: vi.fn() }));

const root = path.resolve(__dirname, '../../..');
const run = (overrides: Partial<ResearchHistorySummary> = {}): ResearchHistorySummary => ({
	run_id: 'research-run-0001', canonical: '2330.TWSE', security_name: '台積電', created_at: '2026-09-17T01:00:00Z', evidence_as_of: '2026-09-16',
	research_version: 'taiwan_ai_research_v2', payload_version: 'taiwan_ai_research_v2', model_provider: 'local', model_name: 'test-model',
	completeness: 'complete', stale: false, partial: false, has_previous: true, ...overrides,
});

const comparison = (overrides: Partial<ResearchComparison> = {}): ResearchComparison => ({
	comparable: true, current_run_id: 'research-run-0002', previous_run_id: 'research-run-0001',
	summary: { unchanged: 4, changed: 1, newly_available: 1, no_longer_available: 1, stale: 0, partial: 0, unavailable: 0, corporate_events_added: 1, corporate_events_removed: 0 },
	domains: [
		{ domain: 'price', state: 'changed', changed_fields: [{ path: 'price.return_5d_percent', state: 'changed', before: 1, after: 2, delta: 1, delta_percent: 100 }] },
		{ domain: 'industry', state: 'partial', changed_fields: [] },
		{ domain: 'margin', state: 'unavailable', changed_fields: [] },
	],
	assumptions: [{ kind: 'section', section: 'price', text: '價格依據', evidence_keys: ['price.status'], status: 'still_supported' }],
	corporate_events_added: [{ event_id: 'event-new', title: '新增重大訊息', category: '重大訊息' }], corporate_events_removed: [], ...overrides,
});

describe('Taiwan Research History', () => {
	it('renders the empty state', () => {
		const html = renderToStaticMarkup(<ResearchHistoryList runs={[]} onOpen={() => {}} onCompare={() => {}} />);
		expect(html).toContain('尚無研究歷史');
		expect(html).toContain('結構化證據');
	});

	it('renders history summaries, completeness, versions and comparison availability', () => {
		const html = renderToStaticMarkup(<ResearchHistoryList runs={[run(), run({ run_id: 'research-run-0000', has_previous: false, completeness: 'partial', partial: true })]} activeRunID="research-run-0001" onOpen={() => {}} onCompare={() => {}} />);
		for (const text of ['最新研究', '歷史研究', 'taiwan_ai_research_v2', '證據完整', '部分證據', '與前次研究比較', '沒有更早的成功研究']) expect(html).toContain(text);
	});

	it('renders changed numeric evidence, partial/unavailable states and textual assumption status', () => {
		const html = renderToStaticMarkup(<ResearchComparisonView payload={{ available: true, comparison: comparison() }} />);
		for (const text of ['證據變化摘要', '市場價格', '證據已改變', '1 → 2', '變化 1（100%）', '部分資料', '無法取得', '仍有證據支持', '新增重大訊息']) expect(html).toContain(text);
	});

	it('renders no-previous and incompatible-version states without guessing', () => {
		const noPrevious = renderToStaticMarkup(<ResearchComparisonView payload={{ available: false, reason: 'no previous successful research' }} />);
		expect(noPrevious).toContain('沒有可比較的前次研究');
		const incompatible = renderToStaticMarkup(<ResearchComparisonView payload={{ available: true, comparison: comparison({ comparable: false, reason: 'research history version is not comparable' }) }} />);
		expect(incompatible).toContain('此歷史版本無法完整比較');
	});

	it('includes loading/error accessibility, last-request-wins guards and workspace integration', () => {
		const source = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanResearchHistory.tsx'), 'utf8');
		const workspace = fs.readFileSync(path.join(root, 'frontend/src/components/TaiwanStockResearchWorkspace.tsx'), 'utf8');
		expect(source).toContain('正在讀取研究歷史');
		expect(source).toContain('研究歷史暫時無法取得');
		expect(source).toContain('role="alert"');
		expect(source).toContain('aria-expanded={open}');
		expect(source).toContain('requestGate.current.isCurrent(current)');
		expect(source).toContain('comparisonRequestGate.current.invalidate()');
		expect(source).toContain('返回最新研究');
		expect(source).toContain('載入更多較舊研究');
		expect(workspace).toContain("'X-Research-Run-ID': runID");
		expect(workspace).toContain('<TaiwanResearchHistory');
	});

	it('executes list, record and comparison API helpers and propagates failures', async () => {
		const config = {} as BackendConfig;
		const mockedRequest = vi.mocked(requestJSON);
		mockedRequest.mockResolvedValueOnce({ data: { runs: [run()], total: 1, limit: 20, offset: 0 } });
		await expect(fetchResearchHistoryPage(config, '2330.TWSE')).resolves.toMatchObject({ data: { total: 1 } });
		expect(mockedRequest).toHaveBeenLastCalledWith(config, expect.stringContaining('limit=20&offset=0'));
		mockedRequest.mockResolvedValueOnce({ data: { ...run(), evidence_snapshot: {}, research_result: {}, provenance: {}, validity: {} } });
		await expect(fetchResearchHistoryRecord(config, '2330.TWSE', 'research-run-0001')).resolves.toHaveProperty('data.run_id', 'research-run-0001');
		mockedRequest.mockResolvedValueOnce({ data: { available: true, comparison: comparison() } });
		await expect(fetchResearchComparison(config, '2330.TWSE', 'research-run-0001')).resolves.toHaveProperty('data.available', true);
		mockedRequest.mockRejectedValueOnce(new Error('API unavailable'));
		await expect(fetchResearchHistoryPage(config, '2330.TWSE')).rejects.toThrow('API unavailable');
	});

	it('invalidates stale list/open/comparison responses deterministically', () => {
		const gate = new LatestRequestGate();
		const first = gate.begin();
		const second = gate.begin();
		expect(gate.isCurrent(first)).toBe(false);
		expect(gate.isCurrent(second)).toBe(true);
		gate.invalidate();
		expect(gate.isCurrent(second)).toBe(false);
	});
});
