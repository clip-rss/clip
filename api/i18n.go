package api

import (
	"github.com/clip-rss/clip/internal/i18n"
	"github.com/clip-rss/clip/internal/store"
)

// backendLanguage reads the persisted language at the point a user-visible
// message is created. This also makes runtime language changes take effect
// without rebuilding services.
func backendLanguage(st *store.Store) string {
	if st == nil {
		return i18n.English
	}
	settings, err := st.GetSettings()
	if err != nil {
		return i18n.English
	}
	return settings.Language
}

func backendError(st *store.Store, err error) error {
	return i18n.LocalizeError(backendLanguage(st), err)
}
