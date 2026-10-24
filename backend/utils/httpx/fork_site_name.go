package httpx

// WebhookSiteName labels webhook requests in logs. Fork: the site name is logged verbatim, and a
// webhook URL can carry its token (e.g. Discord), so callers pass this instead of the URL.
const WebhookSiteName = "Webhook"
