package kooky

import "time"

// BrowserKookyReader is an object that allows read access to cookies
// installed on the local operating system.
type BrowserKookyReader interface {
	ReadCookies(
		filename string,
		domainFilter string,
		nameFilter string,
		expireAfter time.Time,
	) ([]*Cookie, error)

	ReadAllCookies(filePath string) ([]*Cookie, error)

	GetDefaultInstallPath(operatingSystem string) (string, error)
	GetDefaultCookieFilePath(operatingSystem string) (string, error)
}
