const fs = require('node:fs');
const path = require('node:path');
const { inspect } = require('node:util');
const DEFAULT_MAX_BYTES = 5 * 1024 * 1024;
const DEFAULT_BACKUPS = 5;

function redactRuntimeLog(value) {
  let text = String(value ?? '');
  text = text.replace(/\bBearer\s+[^\s"',;}]+/gi, 'Bearer <redacted>');
  text = text.replace(/([?&](?:key|token|api[_-]?key|authorization|cookie|credential|password|secret)=)[^&\s"']*/gi, '$1<redacted>');
  text = text.replace(/(["']?\b(?:[\w-]*[_-])?(?:api[_-]?key|token|authorization|cookie|credential|password|secret)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;&}]+)/gi, '$1<redacted>');
  return text.slice(0, 16384);
}
function createRotatingLogger({ directory, fileName, component = 'runtime', maxBytes = DEFAULT_MAX_BYTES, backups = DEFAULT_BACKUPS, mirror, now = () => new Date() }) {
  if (!directory || !fileName || path.basename(fileName) !== fileName) throw new Error('Invalid log destination');
  const target = path.join(path.resolve(directory), fileName);
  const limit = Math.max(64, Number(maxBytes) || DEFAULT_MAX_BYTES);
  const retained = Math.max(0, Math.min(20, Math.floor(backups)));
  const exists = (name) => { try { return fs.statSync(name).size; } catch (error) { if (error.code === 'ENOENT') return 0; throw error; } };
  const write = (level, feature, values) => {
    const message = redactRuntimeLog(values.map((value) => typeof value === 'string' ? value : inspect(value, { depth: 4, maxArrayLength: 40, getters: false })).join(' '));
    const prefix = now().toISOString() + ' ' + String(level).replace(/[^a-z]/g, '') + ' ' + String(component).slice(0, 40) + ' ' + String(feature).replace(/[^\w.-]/g, '').slice(0, 80) + ' ';
    let record = Buffer.from(prefix + message + '\n');
    if (record.length > limit) record = Buffer.concat([record.subarray(0, limit - 1), Buffer.from('\n')]);
    try {
      fs.mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
      if (exists(target) + record.length > limit) {
        if (retained === 0) fs.writeFileSync(target, '', { mode: 0o600 });
        else {
          for (let index = retained; index > 0; index--) {
            const previous = index === 1 ? target : target + '.' + (index - 1);
            const next = target + '.' + index;
            if (exists(previous)) { fs.rmSync(next, { force: true }); fs.renameSync(previous, next); }
          }
        }
      }
      fs.appendFileSync(target, record, { mode: 0o600 });
    } catch { mirror?.error?.('Runtime log unavailable'); }
    mirror?.[level === 'error' ? 'error' : level === 'warn' ? 'warn' : 'log']?.(message);
  };
  const logger = { path: target, directory: path.dirname(target), event: (level, feature, ...values) => write(level, feature, values) };
  for (const level of ['debug', 'info', 'log', 'warn', 'error']) logger[level] = (...values) => write(level === 'log' ? 'info' : level, component, values);
  return logger;
}
module.exports = { DEFAULT_MAX_BYTES, DEFAULT_BACKUPS, createRotatingLogger, redactRuntimeLog };