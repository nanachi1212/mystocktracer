import { expect, test } from 'vitest';
import { updatePrimaryAction } from './AppUpdatePanel';
import type { AppUpdateStatus } from '../lib/backend';

const actions: [AppUpdateStatus['state'], string, string][] = [
  ['idle', 'check', 'check'],
  ['error', 'check', 'check'],
  ['disabled', 'check', 'check'],
  ['checking', 'check', 'check'],
  ['not-available', 'check', 'check'],
  ['available', 'download', 'release'],
  ['downloading', 'check', 'check'],
  ['downloaded', 'install', 'release'],
  ['installing', 'check', 'check'],
];
for (const [state, automatic, manual] of actions) {
  for (const mode of ['automatic', 'manual'] as const) {
    test('update action for ' + state + ' in ' + mode + ' mode', () => {
      const status: AppUpdateStatus = { state, installMode: mode, supported: state !== 'disabled', currentVersion: '0.9.2', progress: 0, message: '' };
      expect(updatePrimaryAction(status)).toBe(mode === 'manual' ? manual : automatic);
    });
  }
}
