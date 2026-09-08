const ALLOWED_SUBSCRIPTION_AI_URLS = new Set([
  'https://chatgpt.com/',
  'https://claude.ai/',
  'https://gemini.google.com/',
]);

function validateSubscriptionAIURL(targetUrl) {
  if (typeof targetUrl !== 'string' || !ALLOWED_SUBSCRIPTION_AI_URLS.has(targetUrl)) {
    throw new Error(`不允許開啟的外部網址: ${targetUrl}`);
  }
  return true;
}

module.exports = {
  ALLOWED_SUBSCRIPTION_AI_URLS,
  validateSubscriptionAIURL,
};
