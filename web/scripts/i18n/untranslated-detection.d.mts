export const BRAND_AND_LITERAL_KEYS: ReadonlySet<string>

export function isLikelyUntranslated(options: {
  locale: string
  baseValue: unknown
  value: unknown
}): boolean
