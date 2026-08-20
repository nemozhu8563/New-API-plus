import { api } from '@/lib/api'

import type { LegalDocumentResponse } from './types'

export async function getUserAgreement(locale: string) {
  const res = await api.get<LegalDocumentResponse>('/api/user-agreement', {
    params: { locale },
  })
  return res.data
}

export async function getPrivacyPolicy(locale: string) {
  const res = await api.get<LegalDocumentResponse>('/api/privacy-policy', {
    params: { locale },
  })
  return res.data
}
