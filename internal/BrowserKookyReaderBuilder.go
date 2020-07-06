package internal

import (
	"errors"

	kooky "github.com/kgoins/kooky/pkg"
	"github.com/kgoins/kooky/pkg/chrome"
	"github.com/kgoins/kooky/pkg/firefox"
	"github.com/kgoins/kooky/pkg/safari"
)

// BuildBrowserKookyReader constructs the appropriate reader based on the requested browser type.
func BuildBrowserKookyReader(browserType string) (kooky.BrowserKookyReader, error) {
	switch browserType {
	case "firefox":
		return firefox.NewCookieReader(), nil
	case "chrome":
		return chrome.NewCookieReader(), nil
	case "safari":
		return safari.NewCookieReader(), nil
	default:
		return nil, errors.New("Unsupported browser type")
	}
}
