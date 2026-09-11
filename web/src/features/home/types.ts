export interface HomePageContentResponse {
  success?: boolean
  data?: string
  message?: string
}

export interface HomePageContentResult {
  content: string
  isLoaded: boolean
  isUrl: boolean
}
