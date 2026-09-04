import type { ReactElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { InterpretationView, type Component, type Intelligence } from './TaiwanStockResearchWorkspace';
import { TaiwanStockResearchErrorBoundary } from './TaiwanStockResearchErrorBoundary';

const component = (overrides: Partial<Component> = {}): Component => ({
	state: 'positive', status: 'available', freshness: 'fresh', as_of: '2026-09-03', reasons: ['reason'],
	...overrides,
});

type DataQuality = NonNullable<Intelligence['interpretation']>['data_quality'];

const baseIntelligence = (components: Record<string, Component>, dataQuality: DataQuality): Intelligence => ({
	model_version: 'taiwan_stock_intelligence_v1', symbol: '2330.TWSE',
	identity: { canonical_symbol: '2330.TWSE', code: '2330', name: '台積電', exchange: 'TWSE', currency: 'TWD', security_type: 'stock' },
	quote: { status: 'available' },
	price_history_summary: { status: 'available', return_5d_percent: 1, return_20d_percent: 2 },
	fundamentals: { status: 'available' }, institutional: { status: 'available' }, margin: { status: 'available' },
	market_context: { status: 'available' }, industry_context: { status: 'available' },
	interpretation: { model_version: 'taiwan_stock_interpretation_v1', components, data_quality: dataQuality },
});

const fullComponents: Record<string, Component> = {
	price: component(), market: component(), price_market_relationship: component(),
	industry: component(), institutional: component(), margin: component(), fundamentals: component(),
};

describe('InterpretationView data_quality null handling', () => {
	it('renders the exact-null-crash fixture (2330-like: every component available, empty categories serialized as null) without throwing', () => {
		const intelligence = baseIntelligence(fullComponents, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'institutional', 'margin', 'fundamentals'],
			indeterminate_components: null, unavailable_components: null, stale_components: null, partial_components: null,
		});
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('可用 7 項');
		expect(html).toContain('無法判定 0 項');
		expect(html).toContain('無法取得／不適用 0 項');
	});

	it('renders a 6488-like (TPEx) fixture with a mix of indeterminate and unavailable components', () => {
		const components: Record<string, Component> = {
			...fullComponents,
			institutional: component({ state: 'indeterminate', status: 'data_insufficient', reasons: ['latest official institutional flow is unavailable'] }),
			margin: component({ state: 'indeterminate', status: 'unavailable', reasons: ['margin section is unavailable'] }),
		};
		const intelligence = baseIntelligence(components, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'fundamentals'],
			indeterminate_components: ['institutional'], unavailable_components: ['margin'],
			stale_components: null, partial_components: null,
		});
		intelligence.identity = { ...intelligence.identity, canonical_symbol: '6488.TPEX', code: '6488', exchange: 'TPEX' };
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('可用 5 項');
		expect(html).toContain('無法判定 1 項');
		expect(html).toContain('無法取得／不適用 1 項');
	});

	it('renders a 0050 ETF fixture where industry/fundamentals are not_applicable, not fabricated company data', () => {
		const components: Record<string, Component> = {
			...fullComponents,
			industry: component({ state: 'not_applicable', status: 'not_applicable', reasons: ['industry interpretation is not applicable to non-stock security types'] }),
			fundamentals: component({ state: 'not_applicable', status: 'not_applicable', reasons: ['ordinary-stock fundamentals are not applicable'] }),
		};
		const intelligence = baseIntelligence(components, {
			available_components: ['price', 'market', 'price_market_relationship', 'institutional', 'margin'],
			indeterminate_components: null, unavailable_components: ['industry', 'fundamentals'],
			stale_components: null, partial_components: null,
		});
		intelligence.identity = { canonical_symbol: '0050.TWSE', code: '0050', name: '元大台灣50', exchange: 'TWSE', currency: 'TWD', security_type: 'etf' };
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('無法取得／不適用 2 項');
	});

	it('still counts stale and partial categories correctly when populated', () => {
		const intelligence = baseIntelligence(fullComponents, {
			available_components: ['price', 'market', 'price_market_relationship', 'industry', 'institutional', 'margin', 'fundamentals'],
			indeterminate_components: [], unavailable_components: [],
			stale_components: ['price'], partial_components: ['institutional'],
		});
		const html = renderToStaticMarkup(<InterpretationView intelligence={intelligence} />);
		expect(html).toContain('資料較舊 1 項');
		expect(html).toContain('部分資料 1 項');
	});
});

describe('TaiwanStockResearchErrorBoundary', () => {
	// renderToStaticMarkup (React's legacy SSR renderer, the only renderer this repo's test setup
	// exercises without adding a DOM/testing-library dependency) does not run error-boundary
	// recovery the way client rendering does — a thrown child still aborts the whole render.
	// So the catch/fallback contract is verified directly against the boundary's own lifecycle
	// methods instead of through a full render pass.
	it('getDerivedStateFromError flips the boundary into its error state', () => {
		expect(TaiwanStockResearchErrorBoundary.getDerivedStateFromError()).toEqual({ hasError: true });
	});

	it('renders the Traditional Chinese fallback with a reload action once in the error state', () => {
		const instance = new TaiwanStockResearchErrorBoundary({ children: <div>正常內容</div> });
		instance.state = { hasError: true };
		const html = renderToStaticMarkup(instance.render() as ReactElement);
		expect(html).toContain('個股研究畫面發生錯誤，請重新載入此研究。');
		expect(html).toContain('重新載入');
	});

	it('renders children normally when there is no error', () => {
		const html = renderToStaticMarkup(
			<TaiwanStockResearchErrorBoundary>
				<div>正常內容</div>
			</TaiwanStockResearchErrorBoundary>,
		);
		expect(html).toContain('正常內容');
	});
});
