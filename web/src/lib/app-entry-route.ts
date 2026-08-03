export const DEFAULT_CONSOLE_ROUTE = '/dashboard/overview' as const

export function resolveAppEntryRoute(isAuthenticated: boolean) {
  return isAuthenticated ? DEFAULT_CONSOLE_ROUTE : ('/sign-in' as const)
}
