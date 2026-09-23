// Research citations may leave the app through the system browser only.
function externalLinkURL(value) {
  try {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return null;
    return url.href;
  } catch { return null; }
}
function externalWindowHandler(openExternal, onError) {
  return ({ url }) => {
    const target = externalLinkURL(url);
    if (target) Promise.resolve().then(() => openExternal(target)).catch(onError);
    return { action: 'deny' };
  };
}
module.exports = { externalLinkURL, externalWindowHandler };
