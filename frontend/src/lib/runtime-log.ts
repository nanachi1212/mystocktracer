type RuntimeLogLevel = 'debug' | 'info' | 'warn' | 'error';

const listening = new WeakSet<Window>();
const routes: ReadonlyArray<readonly [string, string]> = [
  ['/api/v1/tw/', 'taiwan'], ['/api/v1/settings', 'settings'],
  ['/api/v1/ai', 'ai-chat'], ['/api/health', 'health'],
];

export function runtimeFeatureForPath(path: string): string {
  return routes.find(([prefix]) => path.startsWith(prefix))?.[1] ?? 'renderer';
}

export function runtimeErrorDetails(error: unknown) {
  if (!(error instanceof Error)) {
    return { name: 'Error', message: typeof error === 'string' ? error : 'Unknown runtime error' };
  }
  return { name: error.name, message: error.message, stack: (error.stack ?? '').split('\n').slice(0, 8).join('\n') };
}

export function sanitizeRuntimeMessage(input: string): string {
  const withoutURLSecrets = input.replace(/https?:\/\/[^\s"']+/gi, (match) => {
    const boundary = match.search(/[?#]/);
    return boundary < 0 ? match : match.slice(0, boundary);
  });
  const withoutBearer = withoutURLSecrets.replace(/\bBearer\s+[^\s"',;}]+/gi, 'Bearer <redacted>');
  return withoutBearer.replace(
    /(["']?\b(?:[\w-]*[_-])?(?:api[_-]?key|token|authorization|cookie|credential|password|secret)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;&}]+)/gi,
    (_, label: string) => `${label}<redacted>`,
  ).slice(0, 8192);
}

function diagnosticText(details: unknown): string {
  if (typeof details === 'string') return details;
  try { return JSON.stringify(details) ?? ''; }
  catch { return 'Unserializable runtime diagnostic'; }
}

export function logRuntimeEvent(level: RuntimeLogLevel, feature: string, details: unknown) {
  if (typeof window === 'undefined') return;
  const send = window.mystocktracer?.logRuntimeEvent;
  if (!send) return;
  try {
    void send({ level, feature, message: sanitizeRuntimeMessage(diagnosticText(details)) }).catch(() => undefined);
  } catch {
    // Diagnostics must never interrupt the renderer.
  }
}

export function installRuntimeLogging() {
  if (typeof window === 'undefined' || listening.has(window)) return;
  listening.add(window);
  window.addEventListener('error', (event) => {
    logRuntimeEvent('error', 'renderer', { ...runtimeErrorDetails(event.error || event.message), line: event.lineno, column: event.colno });
  });
  window.addEventListener('unhandledrejection', (event) => {
    logRuntimeEvent('error', 'renderer', runtimeErrorDetails(event.reason));
  });
  logRuntimeEvent('info', 'renderer', { event: 'renderer_start' });
}
