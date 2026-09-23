const test = require('node:test');
const assert = require('node:assert/strict');
const { completeShutdown } = require('../shutdown.cjs');

test('shutdown awaits cleanup and quits even when cleanup or reporting rejects', async () => {
  for (const outcome of ['success', 'cleanup-error', 'report-error']) {
    const calls = [];
    const result = completeShutdown({
      stop: async () => { await Promise.resolve(); calls.push('stop'); if (outcome !== 'success') throw new Error('cleanup'); },
      report: error => { assert.equal(error.message, 'cleanup'); calls.push('report'); if (outcome === 'report-error') throw new Error('report'); },
      quit: () => calls.push('quit'),
    });
    if (outcome === 'report-error') await assert.rejects(result, /report/); else await result;
    assert.deepEqual(calls, outcome === 'success' ? ['stop', 'quit'] : ['stop', 'report', 'quit']);
  }
});
