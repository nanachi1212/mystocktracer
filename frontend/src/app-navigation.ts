import { Activity, BarChart3, Bell, BrainCircuit, Gauge, LayoutDashboard, Search, Star, WalletCards, type LucideIcon } from 'lucide-react';
import { taiwanMarketDetailNavigation, taiwanPrimaryNavigation } from './lib/taiwan-product';

export type WorkspaceMode =
	| 'taiwan-dashboard'
	| 'taiwan-overview'
	| 'taiwan-breadth'
	| 'taiwan-emotion'
	| 'taiwan-industry'
	| 'taiwan-screener'
	| 'taiwan-stock'
	| 'taiwan-watchlist'
	| 'taiwan-portfolio'
	| 'taiwan-alerts'
	| 'ai';

export const workspaceIcons: Record<Exclude<WorkspaceMode, 'ai'>, LucideIcon> = {
	'taiwan-dashboard': LayoutDashboard,
	'taiwan-overview': BarChart3,
	'taiwan-breadth': Activity,
	'taiwan-emotion': Gauge,
	'taiwan-industry': BarChart3,
	'taiwan-screener': Search,
	'taiwan-stock': BrainCircuit,
	'taiwan-watchlist': Star,
	'taiwan-portfolio': WalletCards,
	'taiwan-alerts': Bell,
};

export const workspaceTitles: Record<Exclude<WorkspaceMode, 'ai'>, { title: string; description: string }> = {
	'taiwan-dashboard': { title: '今日總覽', description: '市場、持股、自選股、事件與研究活動的資訊中心' },
	'taiwan-overview': { title: '台股總覽', description: '上市、上櫃行情與官方市場資料' },
	'taiwan-breadth': { title: '市場廣度', description: '上漲、下跌家數與成交方向' },
	'taiwan-emotion': { title: '市場情緒', description: '以確定性規則呈現市場參與與訊號分歧' },
	'taiwan-industry': { title: '產業雷達', description: '官方產業分類的相對市場廣度與成交方向' },
	'taiwan-screener': { title: '台股選股器', description: '以官方市場快照篩選、排序台灣證券' },
	'taiwan-stock': { title: '個股分析', description: '官方證據與確定性解讀，不提供投資推薦' },
	'taiwan-watchlist': { title: '自選股', description: '已儲存的台灣證券與最新報價' },
	'taiwan-portfolio': { title: '持倉總覽', description: '台股市值、成本、損益與集中度' },
	'taiwan-alerts': { title: '事件提醒', description: '共用公司事件收件匣與已讀狀態' },
};

export const primaryNavigation = taiwanPrimaryNavigation;
export const marketNavigation = taiwanMarketDetailNavigation;
