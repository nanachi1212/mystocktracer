module.exports = Object.freeze({
  name: 'mystocktracer',
  appId: 'com.nanachi1212.mystocktracer',
  backendName: (platform = process.platform) => `mystocktracer-backend${platform === 'win32' ? '.exe' : ''}`,
});