import { Component, type ErrorInfo, type ReactNode } from 'react';

type Props = { children: ReactNode };
type State = { hasError: boolean };

// Minimal-scope boundary around Taiwan stock research render output only. App shell, sidebar
// navigation, and stock selector all live outside this boundary in App.tsx, so a render failure
// here never takes down the rest of the app.
export class TaiwanStockResearchErrorBoundary extends Component<Props, State> {
	state: State = { hasError: false };

	static getDerivedStateFromError(): State {
		return { hasError: true };
	}

	componentDidCatch(error: unknown, info: ErrorInfo) {
		console.error('Taiwan stock research render failed', error, info.componentStack);
	}

	render() {
		if (this.state.hasError) {
			return <div className="taiwan-research-error-boundary">
				<strong>個股研究畫面發生錯誤，請重新載入此研究。</strong>
				<button type="button" onClick={() => this.setState({ hasError: false })}>重新載入</button>
			</div>;
		}
		return this.props.children;
	}
}
