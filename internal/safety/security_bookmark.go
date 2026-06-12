package safety

import (
	"fmt"
	"strings"
)

type SecurityBookmark struct {
	Data      string
	Path      string
	CreatedAt string
	Stale     bool
}

type SecurityBookmarkAccess struct {
	Path    string
	Stale   bool
	Started bool
	closeFn func()
}

func (a *SecurityBookmarkAccess) Close() {
	if a == nil || a.closeFn == nil {
		return
	}
	a.closeFn()
	a.closeFn = nil
}

func securityBookmarkRequired(data string) bool {
	return strings.TrimSpace(data) != ""
}

func validateSecurityBookmark(data string) error {
	if strings.TrimSpace(data) == "" {
		return fmt.Errorf("security bookmark data is empty")
	}
	return nil
}
