import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

afterEach(() => vi.unstubAllGlobals());

function renderApp(hash: string) {
	vi.stubGlobal('window', { location: { hash } });
	return renderToStaticMarkup(<App />);
}

describe('Taiwan navigation simplification', () => {
	it('renders four primary buttons followed by three secondary market-detail buttons', () => {
		const html = renderApp('#taiwan-overview');
		const nav = html.match(/<aside\b[^>]*>[\s\S]*?<nav>([\s\S]*?)<\/nav>/)![1];
		const primary = nav.match(/class="sidebar-primary"[^>]*>([\s\S]*?)<\/div>/)![1];
		const details = nav.match(/class="sidebar-market-details"[^>]*>([\s\S]*?)<\/div>/)![1];
		const labels = (markup: string) => [...markup.matchAll(/<button\b[^>]*aria-label="([^"]+)"/g)].map((match) => match[1]);
		expect(labels(primary)).toEqual(['台股總覽', '台股選股器', '自選股', '個股分析']);
		expect(labels(details)).toEqual(['市場廣度', '市場情緒', '產業雷達']);
		expect(nav.match(/<button\b/g)).toHaveLength(7);
		expect(nav).not.toContain('AI 研究');
		expect(nav).not.toContain('個股研究');
		expect(nav).toContain('role="group" aria-label="市場詳情"');
		expect(details).toContain('class="sidebar-group-heading">市場詳情');
		// Icon-only buttons retain accessible names and tooltips when CSS hides their text.
		for (const button of nav.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)) {
			expect(button[1]).toMatch(/title="[^"]+"/);
			expect(button[1]).toMatch(/aria-label="[^"]+"/);
			expect(button[2]).toContain('<svg');
		}
	});

	it('renders the real stock workspace with the analysis title and active button for an old bookmark', () => {
		const html = renderApp('#taiwan-research');
		expect(html).toBe(renderApp('#taiwan-stock'));
		expect(html).toContain('<h1>個股分析</h1>');
		expect(html).toMatch(/<button[^>]*class="active"[^>]*aria-current="page"[^>]*aria-label="個股分析"/);
		expect(html).toContain('class="taiwan-product-workspace taiwan-stock-research"');
		expect(html).toContain('選擇台灣證券開始分析');
		expect(html).toContain('aria-label="台灣證券名稱或代碼"');
		expect(html).not.toContain('個股研究');
	});

	it.each([
		['taiwan-overview', '台股總覽'], ['taiwan-screener', '台股選股器'], ['taiwan-watchlist', '自選股'],
		['taiwan-breadth', '市場廣度'], ['taiwan-emotion', '市場情緒'], ['taiwan-industry', '產業雷達'],
	])('restores #%s with its own title and active navigation', (id, label) => {
		const html = renderApp(`#${id}`);
		expect(html).toContain(`<h1>${label}</h1>`);
		expect(html).toMatch(new RegExp(`<button[^>]*class="active"[^>]*aria-current="page"[^>]*aria-label="${label}"`));
	});
});
