import { createFileRoute, redirect } from '@tanstack/react-router'

import { resolveAppEntryRoute } from '@/lib/app-entry-route'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    throw redirect({
      href: resolveAppEntryRoute(Boolean(auth.user && auth.accessToken)),
      replace: true,
    })
  },
})
