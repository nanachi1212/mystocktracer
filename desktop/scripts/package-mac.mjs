// Compatibility entry point delegates to the canonical product build.
process.argv.splice(2, process.argv.length - 2, 'mac', 'dir');
await import('./electron-builder.mjs');