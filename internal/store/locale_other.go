//go:build !windows

package store

import "os"

func detectSystemLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
