// mystocktracer publishes desktop updates from its own GitHub Releases. Phase B1 replaced the
// inherited easy-stock Alibaba OSS feed, which was controlled by the upstream project: a packaged
// mystocktracer build must never trust an update source someone else owns.
//
// The owner/repo pair is a hard constant on purpose. There is no environment override and no
// configurable URL, so no request parameter, settings value, or hijacked env var can redirect the
// updater at another repository or an arbitrary host. electron-updater's GitHub provider fetches
// latest.yml / latest-mac.yml over HTTPS from this repository's releases and verifies every
// downloaded artifact against the SHA-512 digest recorded there.
const UPDATE_REPOSITORY_OWNER = 'nanachi1212';
const UPDATE_REPOSITORY_NAME = 'mystocktracer';

function resolveUpdateFeed() {
  return { provider: 'github', owner: UPDATE_REPOSITORY_OWNER, repo: UPDATE_REPOSITORY_NAME };
}

function releasePageURL(version) {
  const tag = String(version || '').trim().replace(/^v/, '');
  const base = `https://github.com/${UPDATE_REPOSITORY_OWNER}/${UPDATE_REPOSITORY_NAME}/releases`;
  return tag ? `${base}/tag/v${encodeURIComponent(tag)}` : base;
}

module.exports = { UPDATE_REPOSITORY_OWNER, UPDATE_REPOSITORY_NAME, resolveUpdateFeed, releasePageURL };
