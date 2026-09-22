import ReactMarkdown from 'react-markdown';
import remarkBreaks from 'remark-breaks';
import remarkGfm from 'remark-gfm';

type Props = { content: string; markdown?: boolean };
const allowedProtocols = new Set(['http:', 'https:', 'mailto:']);

function safeURL(value?: string) {
	if (!value) return undefined;
	try { const url = new URL(value, 'https://mystocktracer.local'); return allowedProtocols.has(url.protocol) ? value : undefined; }
	catch { return undefined; }
}

export function MessageContent({ content, markdown = false }: Props) {
	if (!markdown) return <div className="ai-message-content ai-message-content-plain">{content}</div>;
	return <div className="ai-message-content ai-markdown"><ReactMarkdown remarkPlugins={[remarkGfm, remarkBreaks]} skipHtml components={{ a: ({ children, href }) => { const safe = safeURL(href); return safe ? <a href={safe} target="_blank" rel="noreferrer noopener">{children}</a> : <span>{children}</span>; } }}>{content}</ReactMarkdown></div>;
}
