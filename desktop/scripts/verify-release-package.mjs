import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

import { fileURLToPath } from 'node:url';

export function verifyReleasePackage(packageRoot, platform) {
	if (!packageRoot || !['macos', 'windows'].includes(platform)) {
		throw new Error('Usage: node verify-release-package.mjs <package-root> <macos|windows>');
	}
	if (!fs.existsSync(packageRoot)) throw new Error(`Release package not found: ${packageRoot}`);

	const resourcesRoot = platform === 'macos'
		? path.join(packageRoot, 'Contents', 'Resources', 'resources')
		: path.join(packageRoot, 'resources', 'resources');
	const executable = platform === 'macos'
		? path.join(packageRoot, 'Contents', 'MacOS', 'easy-stock')
		: path.join(packageRoot, 'easy-stock.exe');
	const backend = path.join(resourcesRoot, 'backend', platform === 'windows' ? 'easy-stock-backend.exe' : 'easy-stock-backend');
	const runtimePython = platform === 'windows'
		? path.join(resourcesRoot, 'hermes-runtime', 'python', 'python.exe')
		: path.join(resourcesRoot, 'hermes-runtime', 'venv', 'bin', 'python');
	const requiredPaths = [
		executable,
		backend,
		runtimePython,
		path.join(resourcesRoot, 'frontend', 'dist', 'index.html'),
		path.join(resourcesRoot, 'hermes-runtime', 'runtime-manifest.json'),
		path.join(resourcesRoot, 'agent-browser'),
	];
	for (const requiredPath of requiredPaths) {
		if (!fs.existsSync(requiredPath)) throw new Error(`Release package is incomplete: ${requiredPath}`);
	}
	if (platform === 'macos') verifyMacBundle(packageRoot);
	if (platform === 'windows' && fs.existsSync(path.join(resourcesRoot, 'hermes-runtime', 'venv'))) {
		throw new Error('Windows release still contains the build-only Hermes venv');
	}

	const forbiddenComponents = new Set([
		'.runtime',
		'hermes-home',
		'browser-auth',
		'Local Storage',
		'Session Storage',
		'Partitions',
	]);
	const forbiddenFiles = new Set([
		'.credentials.json',
		'Cookies',
		'Cookies-journal',
		'id_rsa',
		'id_ed25519',
	]);
	const violations = [];
	walk(packageRoot, (entryPath, entry) => {
		const name = entry.name;
		if (forbiddenComponents.has(name) || forbiddenFiles.has(name)) violations.push(path.relative(packageRoot, entryPath));
		if (name === '.env' || (name.startsWith('.env.') && name !== '.env.example')) violations.push(path.relative(packageRoot, entryPath));
		if (/\.(?:db|db-shm|db-wal|sqlite|sqlite3)$/i.test(name)) violations.push(path.relative(packageRoot, entryPath));
	});
	if (violations.length) {
		throw new Error(`Release package contains local or sensitive runtime files:\n${violations.map((item) => `- ${item}`).join('\n')}`);
	}
	verifyAppAsar(packageRoot, platform);
	verifyBundledPython(packageRoot, runtimePython, platform);
	console.log(`Release package verified: ${packageRoot}`);
}

const currentFilePath = fileURLToPath(import.meta.url);
if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(currentFilePath)) {
	const packageRoot = path.resolve(process.argv[2] || '');
	const platform = process.argv[3];
	verifyReleasePackage(packageRoot, platform);
}

function verifyMacBundle(appPath) {
	const invalidLinks = [];
	walk(appPath, (entryPath, entry) => {
		if (!entry.isSymbolicLink()) return;
		const target = fs.readlinkSync(entryPath);
		if (path.isAbsolute(target) || !fs.existsSync(entryPath)) {
			invalidLinks.push(`${path.relative(appPath, entryPath)} -> ${target}`);
		}
	});
	if (invalidLinks.length) {
		throw new Error(`macOS release contains invalid framework symlinks:\n${invalidLinks.map((item) => `- ${item}`).join('\n')}`);
	}
	const result = spawnSync('codesign', ['--verify', '--deep', '--strict', appPath], { encoding: 'utf8' });
	if (result.error) throw result.error;
	if (result.status !== 0) {
		throw new Error(`macOS release failed code-signature validation: ${(result.stderr || result.stdout || '').trim()}`);
	}
}

function verifyBundledPython(packageRoot, python, platform) {
	const script = 'import hermes_cli, tui_gateway';
	// Windows executes the copied base interpreter directly, so isolated mode
	// proves it does not resolve packages from the build runner. The macOS
	// launcher intentionally supplies its package-local PYTHONPATH.
	const args = platform === 'windows' ? ['-I', '-c', script] : ['-c', script];
	const result = spawnSync(python, args, {
		cwd: packageRoot,
		encoding: 'utf8',
		env: {
			...process.env,
			PYTHONNOUSERSITE: '1',
			PYTHONDONTWRITEBYTECODE: '1',
			ENABLE_MCP: '0',
			SKIP_BACKGROUND_TASKS: '1',
		},
	});
	if (result.error) throw result.error;
	if (result.status !== 0) {
		throw new Error(`Bundled Python runtime failed its isolated import check: ${(result.stderr || '').trim() || `exit status ${result.status}`}`);
	}
}

function walk(root, visit) {
	for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
		const entryPath = path.join(root, entry.name);
		visit(entryPath, entry);
		if (entry.isDirectory()) walk(entryPath, visit);
	}
}

export function listAsarFiles(archivePath) {
	const fd = fs.openSync(archivePath, 'r');
	try {
		const sizeBuf = Buffer.alloc(16);
		fs.readSync(fd, sizeBuf, 0, 16, 0);
		const headerSize = sizeBuf.readUInt32LE(12);
		const headerBuf = Buffer.alloc(headerSize);
		fs.readSync(fd, headerBuf, 0, headerSize, 16);
		const header = JSON.parse(headerBuf.toString('utf8'));
		const files = new Set();
		function traverse(node, prefix = '') {
			if (!node.files) return;
			for (const [name, entry] of Object.entries(node.files)) {
				const full = prefix ? `${prefix}/${name}` : name;
				files.add(full);
				if (entry.files) traverse(entry, full);
			}
		}
		traverse(header);
		return files;
	} finally {
		fs.closeSync(fd);
	}
}

export function requiredLocalRuntimeModules() {
	return [
		'main.cjs',
		'preload.cjs',
		'review-login-preload.cjs',
		'xueqiu-login-preload.cjs',
		'backend-process.cjs',
		'browser-auth.cjs',
		'data-protection.cjs',
		'hermes-runtime-root.cjs',
		'runtime-logger.cjs',
		'subscription-ai-url.cjs',
		'taoguba-browser-bridge.cjs',
		'update-feed.cjs',
		'update-manager.cjs',
		'user-data.cjs',
		'xueqiu-browser-bridge.cjs',
	];
}

export function verifyAppAsar(appRoot, targetPlatform) {
	const asarPath = targetPlatform === 'macos'
		? path.join(appRoot, 'Contents', 'Resources', 'app.asar')
		: path.join(appRoot, 'resources', 'app.asar');
	if (!fs.existsSync(asarPath)) {
		throw new Error(`Packaged app.asar not found: ${asarPath}`);
	}
	const files = listAsarFiles(asarPath);
	const missing = requiredLocalRuntimeModules().filter((mod) => !files.has(mod));
	if (missing.length > 0) {
		throw new Error(`Packaged app.asar is missing required local runtime modules:\n${missing.map((item) => `- ${item}`).join('\n')}`);
	}
}
