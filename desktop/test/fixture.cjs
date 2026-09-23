const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

// Synthetic filesystem state only, with automatic cleanup even after assertions fail.
module.exports = function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mystocktracer-contract-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true, maxRetries: 4, retryDelay: 100 }));
  const at = (...parts) => path.join(root, ...parts);
  const put = (relative, value = 'fixture') => {
    const file = at(relative);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, value);
    return file;
  };
  return { root, at, put, read: relative => fs.readFileSync(at(relative)) };
};
