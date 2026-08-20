import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import zh from './locales/zh.json'
import zhTW from './locales/zh-TW.json'
import en from './locales/en.json'

export function initialLanguage(): 'zh' | 'zh-TW' | 'en' {
  const preferredLanguages = navigator.languages
  const languages = preferredLanguages?.length
    ? preferredLanguages
    : [navigator.language]
  for (const raw of languages) {
    const lang = raw.toLowerCase().replace(/_/g, '-').split(/[.@]/, 1)[0]
    if (
      lang === 'zh-tw' ||
      lang.startsWith('zh-tw-') ||
      lang === 'zh-hk' ||
      lang.startsWith('zh-hk-') ||
      lang === 'zh-mo' ||
      lang.startsWith('zh-mo-') ||
      lang.startsWith('zh-hant')
    ) {
      return 'zh-TW'
    }
    if (lang === 'zh' || lang.startsWith('zh-')) return 'zh'
  }
  return 'en'
}

i18next.use(initReactI18next).init({
  resources: {
    zh: { translation: zh },
    'zh-TW': { translation: zhTW },
    en: { translation: en },
  },
  lng: initialLanguage(),
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  returnObjects: false,
  returnNull: false,
})

export default i18next
