import path from 'node:path';

const PACKAGE_RESOURCES_ENV = 'A_STOCK_PACKAGE_RESOURCES_DIR';

export function resolvePackageResourcesDir({ desktopRoot, repoRoot, override = process.env[PACKAGE_RESOURCES_ENV] } = {}) {
	if (!desktopRoot || !repoRoot) throw new Error('desktopRoot and repoRoot are required');

	const resolvedDesktopRoot = path.resolve(desktopRoot);
	const resolvedRepoRoot = path.resolve(repoRoot);
	const candidate = path.resolve(
		override
			? (path.isAbsolute(override) ? override : path.join(resolvedRepoRoot, override))
			: path.join(resolvedDesktopRoot, 'dist', 'package-resources'),
	);
	const protectedRoots = [
		resolvedRepoRoot,
		resolvedDesktopRoot,
		path.join(resolvedDesktopRoot, 'dist'),
	];
	const sourceRoots = [
		path.join(resolvedRepoRoot, 'frontend'),
		path.join(resolvedRepoRoot, 'backend'),
		path.join(resolvedDesktopRoot, 'scripts'),
		path.join(resolvedDesktopRoot, 'assets'),
		path.join(resolvedDesktopRoot, 'test'),
		path.join(resolvedDesktopRoot, 'resources'),
	];

	if (protectedRoots.some((protectedRoot) => isSamePath(candidate, protectedRoot))) {
		throw new Error(`${PACKAGE_RESOURCES_ENV} must be a dedicated generated directory: ${candidate}`);
	}
	if (sourceRoots.some((sourceRoot) => isSamePath(candidate, sourceRoot) || isWithin(candidate, sourceRoot))) {
		throw new Error(`${PACKAGE_RESOURCES_ENV} must be a dedicated generated directory outside source directories: ${candidate}`);
	}
	if ([...protectedRoots, ...sourceRoots].some((protectedRoot) => isWithin(protectedRoot, candidate))) {
		throw new Error(`${PACKAGE_RESOURCES_ENV} must not be an ancestor of repository or source directories: ${candidate}`);
	}

	return candidate;
}

function isSamePath(left, right) {
	return path.resolve(left).toLowerCase() === path.resolve(right).toLowerCase();
}

function isWithin(candidate, root) {
	const relative = path.relative(path.resolve(root), path.resolve(candidate));
	return relative !== '' && !relative.startsWith(`..${path.sep}`) && relative !== '..' && !path.isAbsolute(relative);
}
