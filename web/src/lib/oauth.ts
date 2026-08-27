// ============================================================================
// OAuth URL Builders
// ============================================================================

export interface CustomOAuthBinding {
  provider_id: number
  provider_name: string
  provider_slug: string
  provider_icon: string
  provider_user_id: string
}

export function indexCustomOAuthBindings(
  bindings: CustomOAuthBinding[]
): Map<number, CustomOAuthBinding> {
  return new Map(bindings.map((binding) => [binding.provider_id, binding]))
}

/**
 * Build GitHub OAuth URL
 */
export function buildGitHubOAuthUrl(clientId: string, state: string): string {
  return `https://github.com/login/oauth/authorize?client_id=${clientId}&state=${state}&scope=user:email`
}

/**
 * Build Discord OAuth URL
 */
export function buildDiscordOAuthUrl(clientId: string, state: string): string {
  const url = new URL('https://discord.com/oauth2/authorize')
  url.searchParams.set('client_id', clientId)
  url.searchParams.set(
    'redirect_uri',
    `${window.location.origin}/oauth/discord`
  )
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'identify+openid')
  url.searchParams.set('state', state)
  return url.toString()
}

/**
 * Build OIDC OAuth URL
 */
export function buildOIDCOAuthUrl(
  authUrl: string,
  clientId: string,
  state: string,
  serverAddress: string
): string {
  const url = new URL(authUrl)
  const callbackBase = serverAddress.trim().replace(/\/+$/, '')
  if (!callbackBase) {
    throw new Error('OIDC callback server address is empty')
  }
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('redirect_uri', `${callbackBase}/api/oauth/oidc`)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'openid profile email')
  url.searchParams.set('state', state)
  return url.toString()
}

/**
 * Build LinuxDO OAuth URL
 */
export function buildLinuxDOOAuthUrl(clientId: string, state: string): string {
  return `https://connect.linux.do/oauth2/authorize?response_type=code&client_id=${clientId}&state=${state}`
}
