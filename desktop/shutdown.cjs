async function completeShutdown({ stop, report, quit }) {
  try { await stop(); }
  catch (error) { report(error); }
  finally { quit(); }
}
module.exports = { completeShutdown };
