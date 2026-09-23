import type { BackendBridge } from './lib/backend';

declare global {
  interface Window {
    mystocktracer?: BackendBridge;
  }
}

export {};
