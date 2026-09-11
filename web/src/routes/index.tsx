import { createFileRoute } from '@tanstack/react-router'

import { TryvaloLandingPage } from '@/features/landing/tryvalo-landing-page'

export const Route = createFileRoute('/')({
  component: TryvaloLandingPage,
})
