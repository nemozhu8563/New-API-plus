export const PUBLIC_HOME_ROUTE = '/' as const
export const DEFAULT_CONSOLE_ROUTE = '/dashboard/overview' as const

export function resolveLandingPrimaryRoute(isAuthenticated: boolean) {
  return isAuthenticated ? ('/dashboard' as const) : ('/sign-up' as const)
}
