import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import type { KLine, MarketIndexSnapshot, SourceMeta } from '../../lib/backend';
import { CoreIndexView, SourceNotice } from './MarketDataViews';

const meta: SourceMeta = { source: 'TWSE', fetched_at: '2026-09-22T08:00:00+08:00', latency_ms: 8, stale: false };
const index: MarketIndexSnapshot = { id: 'taiex', secid: 'TWSE', code: 'TAIEX', name: '加權指數', region: '台灣',
  market: 'TWSE', currency: 'TWD', price: 24000, change: 120, change_percent: 0.5, status: 'closed', meta };
const sample = (time: string, close: number, change: number): KLine => ({ symbol: 'TAIEX', time, open: close - 5,
  high: close + 10, low: close - 20, close, volume: 1, amount: 1, change_percent: change, meta });

describe('Taiwan index presentation', () => {
  it('shows the selected index, source and recorded session values', () => {
    const html = renderToStaticMarkup(<CoreIndexView indexes={[index]} selectedID="taiex" onSelect={() => {}}
      series={{ index, lines: [sample('2026-09-21', 23800, 0.2), sample('2026-09-22', 24000, 0.5)], meta }}
      seriesLoading={false} meta={meta} locale="zh-TW" />);
    expect(html).toContain('TWSE');
    expect(html).toContain('24,000');
    expect(html).toContain('+0.50%');
    expect(html).toContain('23,780');
    expect(html).toContain('24,010');
    expect(html).toContain('近期交易紀錄');
    expect(html).toContain('aria-label="收盤走勢"');
  });

  it('keeps missing series distinct from a zero return while loading', () => {
    const html = renderToStaticMarkup(<CoreIndexView indexes={[index]} selectedID="taiex" onSelect={() => {}}
      series={null} seriesLoading meta={null} locale="zh-TW" />);
    expect(html).toContain('走勢圖載入中');
    expect(html).toContain('區間報酬');
    expect(html).not.toContain('0.00%');
  });

  it('explains unavailable data and stale fallback provenance', () => {
    const empty = renderToStaticMarkup(<CoreIndexView indexes={[]} selectedID="" onSelect={() => {}}
      series={null} seriesLoading={false} meta={null} locale="zh-TW" />);
    expect(empty).toContain('目前沒有指數資料');
    const notice = renderToStaticMarkup(<SourceNotice meta={{ ...meta, stale: true, fallback_reason: 'backup' }} locale="zh-TW" />);
    expect(notice).toContain('部分資料改用備援來源');
    expect(notice).toContain('stale');
  });
});
