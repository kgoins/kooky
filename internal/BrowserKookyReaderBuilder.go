package internal

import (
	"errors"

	kooky "github.com/kgoins/kooky/pkg"
	"github.com/kgoins/kooky/pkg/firefox"
)

func BuildBrowserKookyReader(browserType string) (kooky.BrowserKookyReader, error) {
	switch browserType {
	case "firefox":
		return firefox.NewCookieReader(), nil
	default:
		return nil, errors.New("Unsupported browser type")
	}
}
