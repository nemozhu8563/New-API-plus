export const BRAND_AND_LITERAL_KEYS = new Set([
  'AI Proxy',
  'AIGC2D',
  'Alipay',
  'Anthropic',
  'API URL',
  'API2GPT',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'Baidu V2',
  'CC Switch',
  'ChatGPT',
  'ChatGPT Subscription (Codex)',
  'Claude',
  'Client ID',
  'Client Secret',
  'Cloudflare',
  'Cohere',
  'DeepSeek',
  'Discord',
  'Doubao Coding Plan',
  'DoubaoVideo',
  'FastGPT',
  'Gemini',
  'Gemini Image 4K',
  'GitHub',
  'Jimeng',
  'JustSong',
  'LingYiWanWu',
  'LinuxDO',
  'MjProxy',
  'MjProxyPlus',
  'Midjourney',
  'MidjourneyPlus',
  'MiniMax',
  'Mistral',
  'MokaAI',
  'Moonshot',
  'New API',
  'New API &lt;noreply@example.com&gt;',
  'NewAPI',
  'OAuth Client Secret',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAIMax',
  'OpenRouter',
  'Pancake',
  'Passkey',
  'Perplexity',
  'QuantumNous',
  'Quota:',
  'Replicate',
  'SiliconFlow',
  'Stripe',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Tencent',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Uptime Kuma',
  'Uptime Kuma URL',
  'Vertex AI',
  'VolcEngine',
  'Waffo Pancake Dashboard',
  'Waffo Pancake MoR',
  'WeChat',
  'WeChat Pay',
  'Webhook URL',
  'Webhook URL:',
  'Well-Known URL',
  'Worker URL',
  'Xinference',
  'Xunfei',
  'Zhipu V4',
  '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"',
  'edit_this',
  'footer.columns.related.links.midjourney',
  'footer.columns.related.links.newApiKeyTool',
  'my-status',
  'neko-api-key-tool',
  'new-api-key-tool',
  'price_xxx',
  'whsec_xxx',
])

export function isLikelyUntranslated({ locale, baseValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (value !== baseValue) return false

  const text = baseValue.trim()
  if (BRAND_AND_LITERAL_KEYS.has(text)) return false
  if (
    /^https?:\/\//.test(text) ||
    /^\/[\w/-]+/.test(text) ||
    /^[\w.-]+@[\w.-]+$/.test(text) ||
    /^smtp\./i.test(text) ||
    /^socks5:/i.test(text) ||
    /^org-/.test(text) ||
    /^gpt-/i.test(text) ||
    /^checkout\./.test(text) ||
    /^footer\./.test(text) ||
    /^[A-Z0-9_ *./:-]+$/.test(text) ||
    text.startsWith('{') ||
    text.startsWith('[') ||
    text.includes('&#10;')
  ) {
    return false
  }
  if (text.length < 6) return false
  if (!/[A-Za-z]{3,}/.test(text)) return false

  if (['ja', 'ru', 'zh', 'zh-TW'].includes(locale)) return true

  if (locale === 'fr' || locale === 'vi') {
    return /\b(the|and|or|to|with|please)\b/i.test(text)
  }

  return false
}
