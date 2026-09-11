import { createContext, useContext, useEffect } from 'react'

import {
  type ContentLayout,
  FIXED_THEME_CUSTOMIZATION,
  type ThemeCustomization,
  type ThemeFont,
  type ThemePreset,
  type ThemeRadius,
  type ThemeScale,
} from '@/lib/theme-customization'

function applyAttribute(name: string, value: string | null) {
  if (typeof document === 'undefined') return
  const body = document.body
  if (!body) return
  if (value === null) {
    body.removeAttribute(name)
  } else {
    body.setAttribute(name, value)
  }
}

type ThemeCustomizationContextType = {
  defaults: ThemeCustomization
  customization: ThemeCustomization
  setPreset: (preset: ThemePreset) => void
  setFont: (font: ThemeFont) => void
  setRadius: (radius: ThemeRadius) => void
  setScale: (scale: ThemeScale) => void
  setContentLayout: (contentLayout: ContentLayout) => void
  resetCustomization: () => void
}

// Fallback used when a consumer renders outside the provider (e.g. an error
// route mounted before providers are ready, or stale HMR boundaries). Keeping
// it permissive prevents the whole tree from crashing — the UI just behaves
// like the defaults until the real provider re-mounts.
const ignoreCustomization = () => undefined

const FIXED_THEME_CONTEXT: ThemeCustomizationContextType = {
  defaults: FIXED_THEME_CUSTOMIZATION,
  customization: FIXED_THEME_CUSTOMIZATION,
  setPreset: ignoreCustomization,
  setFont: ignoreCustomization,
  setRadius: ignoreCustomization,
  setScale: ignoreCustomization,
  setContentLayout: ignoreCustomization,
  resetCustomization: ignoreCustomization,
}

const FALLBACK_CONTEXT: ThemeCustomizationContextType = {
  defaults: FIXED_THEME_CUSTOMIZATION,
  customization: FIXED_THEME_CUSTOMIZATION,
  setPreset: () => {},
  setFont: () => {},
  setRadius: () => {},
  setScale: () => {},
  setContentLayout: () => {},
  resetCustomization: () => {},
}

const ThemeCustomizationContext =
  createContext<ThemeCustomizationContextType>(FALLBACK_CONTEXT)

export function ThemeCustomizationProvider(props: {
  children: React.ReactNode
}) {
  useEffect(() => {
    applyAttribute('data-theme-preset', FIXED_THEME_CUSTOMIZATION.preset)
    applyAttribute('data-theme-font', FIXED_THEME_CUSTOMIZATION.font)
    applyAttribute('data-theme-radius', FIXED_THEME_CUSTOMIZATION.radius)
    applyAttribute('data-theme-scale', FIXED_THEME_CUSTOMIZATION.scale)
    applyAttribute(
      'data-theme-content-layout',
      FIXED_THEME_CUSTOMIZATION.contentLayout
    )
  }, [])

  return (
    <ThemeCustomizationContext.Provider value={FIXED_THEME_CONTEXT}>
      {props.children}
    </ThemeCustomizationContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useThemeCustomization() {
  return useContext(ThemeCustomizationContext)
}
