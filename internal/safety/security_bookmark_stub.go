//go:build !darwin || !cgo

package safety

import "fmt"

func SecurityScopedBookmarksSupported() bool {
	return false
}

func CreateSecurityScopedBookmark(path string) (SecurityBookmark, error) {
	if _, err := ResolveWorkspace(path); err != nil {
		return SecurityBookmark{}, err
	}
	return SecurityBookmark{}, fmt.Errorf("security-scoped bookmarks are only available on macOS builds with cgo")
}

func StartSecurityScopedBookmarkAccess(data string) (*SecurityBookmarkAccess, error) {
	if err := validateSecurityBookmark(data); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("security-scoped bookmarks are only available on macOS builds with cgo")
}
