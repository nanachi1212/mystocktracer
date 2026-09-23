import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { MessageContent } from './MarkdownContent';

const render = (content: string, markdown = true) => renderToStaticMarkup(<MessageContent content={content} markdown={markdown} />);

describe('chat message rendering boundary', () => {
  it('formats assistant tables, lists, code and streamed line breaks', () => {
    const html = render('## 台股摘要\n\n- **資料來源**\n- `TWSE`\n\n| 欄位 | 值 |\n| --- | --- |\n| 市場 | 臺灣 |\n\n第一行\n第二行');
    for (const element of ['<h2>台股摘要</h2>', '<ul>', '<strong>資料來源</strong>', '<code>TWSE</code>', '<table>', '第一行<br/>']) {
      expect(html).toContain(element);
    }
  });

  it('keeps a user message literal', () => {
    const html = render('**不要解析**\n- 原始內容', false);
    expect(html).toContain('**不要解析**');
    expect(html).toContain('- 原始內容');
    expect(html).not.toContain('<strong>');
    expect(html).not.toContain('<ul>');
  });

  it('does not execute HTML or expose an unsafe URL', () => {
    const html = render('<script>private()</script>\n\n[官方頁面](https://example.org/data) [執行](javascript:alert(1)) [嵌入](data:text/html,unsafe)');
    expect(html).not.toContain('<script>');
    expect(html).not.toContain('private()');
    expect(html).toContain('href="https://example.org/data"');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('rel="noreferrer noopener"');
    expect(html).not.toContain('href="javascript:');
    expect(html).not.toContain('href="data:');
  });
});
