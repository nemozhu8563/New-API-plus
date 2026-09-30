import { createContext, useContext, useEffect, useMemo } from 'react'

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

// The fork pins the brand theme: the Anthropic preset (warm cream canvas with
// the clay accent) plus the sans font and large radius/scale are part of the
// product identity, so customization setters are intentionally no-ops.
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

const ThemeCustomizationContext =
  createContext<ThemeCustomizationContextType>(FIXED_THEME_CONTEXT)

export function ThemeCustomizationProvider(props: {
  children: React.ReactNode
}) {
  // Mirror the fixed brand theme to the <body> via data-* attributes so
  // theme-presets.css can override CSS variables at the right cascade layer.
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

  const value = useMemo<ThemeCustomizationContextType>(
    () => FIXED_THEME_CONTEXT,
    []
  )

  return (
    <ThemeCustomizationContext.Provider value={value}>
      {props.children}
    </ThemeCustomizationContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useThemeCustomization() {
  return useContext(ThemeCustomizationContext)
}
