import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
import zh from './locales/zh.json'
import en from './locales/en.json'

function initialLanguage(): 'zh' | 'en' {
  const languages = navigator.languages?.length
    ? navigator.languages
    : [navigator.language]
  return languages.some((lang) =>
    lang.toLowerCase().replace('_', '-').startsWith('zh'),
  )
    ? 'zh'
    : 'en'
}

i18next.use(initReactI18next).init({
  resources: { zh: { translation: zh }, en: { translation: en } },
  lng: initialLanguage(),
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
  returnObjects: false,
  returnNull: false,
})

export default i18next
