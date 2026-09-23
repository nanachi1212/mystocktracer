type RuntimeLogLevel = 'debug' | 'info' | 'warn' | 'error';
const installed = new WeakSet<Window>();

export function runtimeFeatureForPath(value: string) {
  for (const [prefix, name] of [['/api/v1/tw/', 'taiwan'], ['/api/v1/settings', 'settings'], ['/api/v1/ai', 'ai-chat'], ['/api/health', 'health']]) {
    if (value.startsWith(prefix)) return name;
  }
  return 'renderer';
}
export function runtimeErrorDetails(value: unknown) {
  return value instanceof Error
    ? { name: value.name, message: value.message, stack: (value.stack || '').split('\n').slice(0, 8).join('\n') }
    : { name: 'Error', message: typeof value === 'string' ? value : 'Unknown runtime error' };
}
export function sanitizeRuntimeMessage(text: string) {
  return text
    .replace(/(https?:\/\/[^\s?#"']+)[?#][^\s"']*/gi, '$1')
    .replace(/\bBearer\s+[^\s"',;}]+/gi, 'Bearer <redacted>')
    .replace(/(["']?\b(?:[\w-]*[_-])?(?:api[_-]?key|token|authorization|cookie|credential|password|secret)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;&}]+)/gi, '$1<redacted>')
    .slice(0, 8192);
}
export function logRuntimeEvent(level: RuntimeLogLevel, feature: string, details: unknown) {
  const send = globalThis.window?.mystocktracer?.logRuntimeEvent;
  if (!send) return;
  let text: string;
  try { text = typeof details === 'string' ? details : JSON.stringify(details) ?? ''; }
  catch { text = 'Unserializable runtime diagnostic'; }
  // Logging is best effort; rejected log IPC must not recursively log itself.
  void send({ level, feature, message: sanitizeRuntimeMessage(text) }).catch(() => undefined);
}
export function installRuntimeLogging() {
  if (typeof window === 'undefined' || installed.has(window)) return;
  installed.add(window);
  window.addEventListener('error', (event) => {
    logRuntimeEvent('error', 'renderer', { ...runtimeErrorDetails(event.error || event.message), line: event.lineno, column: event.colno });
  });
  window.addEventListener('unhandledrejection', (event) => logRuntimeEvent('error', 'renderer', runtimeErrorDetails(event.reason)));
  logRuntimeEvent('info', 'renderer', { event: 'renderer_start' });
}