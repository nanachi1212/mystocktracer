import type { KLine, MarketIndexSeries, MarketIndexSnapshot, SourceMeta } from '../../lib/backend';

type Locale = 'zh-CN' | 'zh-TW';

const labels = {
  'zh-TW': { source: '來源', fetched: '擷取', backup: '部分資料改用備援來源', empty: '目前沒有指數資料',
    emptyDetail: '等待官方指數資料恢復。', loading: '走勢圖載入中…', noSeries: '目前沒有足夠走勢資料',
    sessions: '個交易日', open: '交易中', closed: '已收盤', unknown: '狀態未知',
    gain: '區間報酬', high: '區間高點', low: '區間低點', time: '行情時間',
    date: '日期', opening: '開盤', highest: '最高', lowest: '最低', closing: '收盤', movement: '漲跌',
    history: '近期交易紀錄', trend: '收盤走勢', choose: '選擇指數' },
  'zh-CN': { source: '来源', fetched: '抓取', backup: '部分数据改用备用来源', empty: '暂无核心指数',
    emptyDetail: '等待指数目录恢复。', loading: '走势图加载中…', noSeries: '暂无足够走势数据',
    sessions: '个交易周期', open: '交易中', closed: '已收盘', unknown: '状态未知',
    gain: '区间收益', high: '区间高点', low: '区间低点', time: '行情时间',
    date: '日期', opening: '开盘', highest: '最高', lowest: '最低', closing: '收盘', movement: '涨跌',
    history: '近期交易记录', trend: '收盘走势', choose: '选择指数' },
} as const;

function price(value: number | undefined, locale: Locale): string {
  if (value === undefined || !Number.isFinite(value) || value === 0) return '--';
  return new Intl.NumberFormat(locale, { minimumFractionDigits: value >= 10_000 ? 0 : 2,
    maximumFractionDigits: value >= 10_000 ? 1 : 2 }).format(value);
}

function percent(value: number | undefined): string {
  if (value === undefined || !Number.isFinite(value)) return '--';
  const digits = value !== 0 && Math.abs(value) < 0.01 ? 4 : 2;
  return `${value > 0 ? '+' : ''}${value.toFixed(digits)}%`;
}

function direction(value: number | undefined) {
  return value === undefined || !Number.isFinite(value) ? 'flat' : value > 0 ? 'up' : value < 0 ? 'down' : 'flat';
}

function timestamp(value: string | undefined, locale: Locale, short = false): string {
  if (!value) return '--';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.valueOf())) return short ? value.slice(0, 10) : value;
  return short ? parsed.toLocaleDateString(locale, { month: '2-digit', day: '2-digit' })
    : parsed.toLocaleString(locale, { hour12: false });
}

export function SourceNotice({ meta, locale = 'zh-CN' }: { meta: SourceMeta | null; locale?: Locale }) {
  if (!meta) return null;
  const label = labels[locale];
  return <div className={`market-source-notice ${meta.stale ? 'stale' : ''}`} role="note">
    <span>{label.source} {meta.source} · {label.fetched} {timestamp(meta.fetched_at, locale)}</span>
    {meta.fallback_reason && <em>{locale === 'zh-TW' ? label.backup : meta.fallback_reason}</em>}
  </div>;
}

function MarketTrend({ rows, locale }: { rows: KLine[]; locale: Locale }) {
  const closes = rows.map((row) => row.close).filter(Number.isFinite);
  if (closes.length < 2) return <p className="index-trend-empty">{labels[locale].noSeries}</p>;
  const floor = Math.min(...closes);
  const span = Math.max(...closes) - floor || 1;
  const coordinates = closes.map((close, position) => {
    const x = position * 100 / (closes.length - 1);
    const y = 88 - (close - floor) * 76 / span;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(' ');
  return <figure className={`index-trend ${direction(closes.at(-1)! - closes[0])}`}>
    <svg viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label={labels[locale].trend}>
      <polyline points={coordinates} fill="none" stroke="currentColor" strokeWidth="2" vectorEffect="non-scaling-stroke" />
    </svg>
    <figcaption><span>{price(floor, locale)}</span><span>{price(Math.max(...closes), locale)}</span></figcaption>
  </figure>;
}

function RecentSessions({ rows, locale }: { rows: KLine[]; locale: Locale }) {
  const label = labels[locale];
  if (rows.length === 0) return <p className="index-trend-empty">{label.noSeries}</p>;
  return <div className="index-session-scroll"><table className="index-session-table">
    <caption>{label.history}</caption>
    <thead><tr>{[label.date, label.opening, label.highest, label.lowest, label.closing, label.movement].map((heading) => <th key={heading} scope="col">{heading}</th>)}</tr></thead>
    <tbody>{rows.slice(-8).reverse().map((row) => <tr key={row.time}>
      <th scope="row">{timestamp(row.time, locale, true)}</th>
      <td>{price(row.open, locale)}</td><td>{price(row.high, locale)}</td><td>{price(row.low, locale)}</td>
      <td>{price(row.close, locale)}</td><td className={direction(row.change_percent)}>{percent(row.change_percent)}</td>
    </tr>)}</tbody>
  </table></div>;
}

export function CoreIndexView({ indexes, selectedID, onSelect, series, seriesLoading, meta, locale = 'zh-CN' }: {
  indexes: MarketIndexSnapshot[];
  selectedID: string;
  onSelect: (id: string) => void;
  series: MarketIndexSeries | null;
  seriesLoading: boolean;
  meta: SourceMeta | null;
  locale?: Locale;
}) {
  const label = labels[locale];
  const active = indexes.find((item) => item.id === selectedID) ?? indexes[0];
  const rows = series && active && series.index.id === active.id ? series.lines : [];
  const first = rows[0]?.close;
  const last = rows.at(-1)?.close;
  const gain = first && last !== undefined ? (last / first - 1) * 100 : undefined;
  const highs = rows.map((row) => row.high).filter((value) => Number.isFinite(value) && value !== 0);
  const lows = rows.map((row) => row.low).filter((value) => Number.isFinite(value) && value !== 0);
  const status = active?.status === 'open' ? label.open : active?.status === 'closed' ? label.closed : label.unknown;

  return <section className="index-workspace">
    <SourceNotice meta={meta} locale={locale} />
    {indexes.length > 0 && <nav className="index-picker" aria-label={label.choose}>
      {indexes.map((item) => <button key={item.id} type="button" aria-current={item.id === active?.id ? 'true' : undefined}
        onClick={() => onSelect(item.id)}>
        <span>{item.name}</span><strong>{price(item.price, locale)}</strong>
        <small className={direction(item.change_percent)}>{percent(item.change_percent)}</small>
      </button>)}
    </nav>}
    {!active ? <div className="index-empty" role="status"><strong>{label.empty}</strong><span>{label.emptyDetail}</span></div> : <>
      <header className="index-summary">
        <div><small>{active.region} · {active.market}</small><h3>{active.name}</h3>
          <p>{rows.length || '--'} {label.sessions} · {status}</p></div>
        <div className="index-summary-value"><strong>{price(active.price, locale)}</strong>
          <span className={direction(active.change_percent)}>{percent(active.change_percent)}</span></div>
      </header>
      <div className="index-insight-grid">
        <div className="index-trend-panel">{seriesLoading ? <p className="index-trend-empty" role="status">{label.loading}</p>
          : <MarketTrend rows={rows} locale={locale} />}</div>
        <dl className="index-stat-grid">
          <div><dt>{label.gain}</dt><dd className={direction(gain)}>{percent(gain)}</dd></div>
          <div><dt>{label.high}</dt><dd>{highs.length ? price(Math.max(...highs), locale) : '--'}</dd></div>
          <div><dt>{label.low}</dt><dd>{lows.length ? price(Math.min(...lows), locale) : '--'}</dd></div>
          <div><dt>{label.time}</dt><dd>{timestamp(series?.index.trade_time ?? active.trade_time, locale)}</dd></div>
        </dl>
      </div>
      <RecentSessions rows={rows} locale={locale} />
    </>}
  </section>;
}
