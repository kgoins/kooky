package chrome

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-sqlite/sqlite3"
	kooky "github.com/kgoins/kooky/pkg"
)

var cookiePathMap kooky.DefaultPathMap
var installLocationPathMap kooky.DefaultPathMap

func init() {
	cookiePathMap = kooky.DefaultPathMap{}
	cookiePathMap.Add("windows", "")
	cookiePathMap.Add("darwin", "")
	cookiePathMap.Add("linux", "")

	installLocationPathMap = kooky.DefaultPathMap{}
	installLocationPathMap.Add("windows", "")
	installLocationPathMap.Add("darwin", "")
	installLocationPathMap.Add("linux", "")
}

// CookieReader implements kooky.KookyReader for the Chrome browser
type CookieReader struct {
	cookiePathMap          kooky.DefaultPathMap
	installLocationPathMap kooky.DefaultPathMap
}

// NewCookieReader returns a new CookieReader
func NewCookieReader() CookieReader {
	return CookieReader{
		cookiePathMap:          cookiePathMap,
		installLocationPathMap: installLocationPathMap,
	}
}

// GetDefaultInstallPath returns the absolute filepath for the default install location on the current OS.
func (reader CookieReader) GetDefaultInstallPath(operatingSystem string) (string, error) {
	path, found := reader.installLocationPathMap.Get(operatingSystem)
	if !found {
		return "", errors.New("Unsupported operating system")
	}

	return path, nil
}

// GetDefaultCookieFilePath returns the absolute filepath for the file used to store cookies on the current OS.
func (reader CookieReader) GetDefaultCookieFilePath(operatingSystem string) (string, error) {
	path, found := reader.cookiePathMap.Get(operatingSystem)
	if !found {
		return "", errors.New("Unsupported operating system")
	}

	return path, nil
}

// ReadAllCookies reads all cookies from the input sqlite database filepath.
func (reader CookieReader) ReadAllCookies(filename string) ([]*kooky.Cookie, error) {
	return reader.ReadCookies(filename, "", "", time.Time{})
}

// ReadCookies reads cookies from the input chrome sqlite database filepath, filtered by the input parameters.
func (reader CookieReader) ReadCookies(filename string, domainFilter string, nameFilter string, expireAfter time.Time) ([]*kooky.Cookie, error) {
	var cookies []*kooky.Cookie
	db, err := sqlite3.Open(filename)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.VisitTableRecords("cookies", func(rowId *int64, rec sqlite3.Record) error {
		if rowId == nil {
			return fmt.Errorf("unexpected nil RowID in Chrome sqlite database")
		}
		cookie := &kooky.Cookie{}

		// TODO(zellyn): handle older, shorter rows?
		if len(rec.Values) < 14 {
			return fmt.Errorf("expected at least 14 columns in cookie file, got: %d", len(rec.Values))
		}

		/*
			-- taken from chrome 80's cookies' sqlite_master
			CREATE TABLE cookies(
				creation_utc INTEGER NOT NULL,
				host_key TEXT NOT NULL,
				name TEXT NOT NULL,
				value TEXT NOT NULL,
				path TEXT NOT NULL,
				expires_utc INTEGER NOT NULL,
				is_secure INTEGER NOT NULL,
				is_httponly INTEGER NOT NULL,
				last_access_utc INTEGER NOT NULL,
				has_expires INTEGER NOT NULL DEFAULT 1,
				is_persistent INTEGER NOT NULL DEFAULT 1,
				priority INTEGER NOT NULL DEFAULT 1,
				encrypted_value BLOB DEFAULT '',
				samesite INTEGER NOT NULL DEFAULT -1,
				source_scheme INTEGER NOT NULL DEFAULT 0,
				UNIQUE (host_key, name, path)
			)
		*/

		domain, ok := rec.Values[1].(string)
		if !ok {
			return fmt.Errorf("expected column 2 (host_key) to to be string; got %T", rec.Values[1])
		}
		name, ok := rec.Values[2].(string)
		if !ok {
			return fmt.Errorf("expected column 3 (name) in cookie(domain:%s) to to be string; got %T", domain, rec.Values[2])
		}
		value, ok := rec.Values[3].(string)
		if !ok {
			return fmt.Errorf("expected column 4 (value) in cookie(domain:%s, name:%s) to to be string; got %T", domain, name, rec.Values[3])
		}
		path, ok := rec.Values[4].(string)
		if !ok {
			return fmt.Errorf("expected column 5 (path) in cookie(domain:%s, name:%s) to to be string; got %T", domain, name, rec.Values[4])
		}

		var expiresUTC int64
		switch i := rec.Values[5].(type) {
		case int64:
			expiresUTC = i
		case int:
			if i != 0 {
				return fmt.Errorf("expected column 6 (expires_utc) in cookie(domain:%s, name:%s) to to be int64 or int with value=0; got %T with value %v", domain, name, rec.Values[5], rec.Values[5])
			}
		default:
			return fmt.Errorf("expected column 6 (expires_utc) in cookie(domain:%s, name:%s) to to be int64 or int with value=0; got %T with value %v", domain, name, rec.Values[5], rec.Values[5])
		}

		encryptedValue, ok := rec.Values[12].([]byte)
		if !ok {
			return fmt.Errorf("expected column 13 (encrypted_value) in cookie(domain:%s, name:%s) to to be []byte; got %T", domain, name, rec.Values[12])
		}

		var expiry time.Time
		if expiresUTC != 0 {
			expiry = chromeCookieDate(expiresUTC)
		}
		creation := chromeCookieDate(*rowId)

		if domainFilter != "" && domain != domainFilter {
			return nil
		}

		if nameFilter != "" && name != nameFilter {
			return nil
		}

		if !expiry.IsZero() && expiry.Before(expireAfter) {
			return nil
		}

		cookie.Domain = domain
		cookie.Name = name
		cookie.Path = path
		cookie.Expires = expiry
		cookie.Creation = creation
		cookie.Secure = rec.Values[6] == 1
		cookie.HttpOnly = rec.Values[7] == 1

		if len(encryptedValue) > 0 {
			decrypted, err := decryptValue(encryptedValue)
			if err != nil {
				return fmt.Errorf("decrypting cookie %v: %v", cookie, err)
			}
			cookie.Value = decrypted
		} else {
			cookie.Value = value
		}
		cookies = append(cookies, cookie)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return cookies, nil

}

// See https://cs.chromium.org/chromium/src/base/time/time.h?l=452&rcl=fceb9a030c182e939a436a540e6dacc70f161cb1
const windowsToUnixMicrosecondsOffset = 11644473600000000

// chromeCookieDate converts microseconds to a time.Time object,
// accounting for the switch to Windows epoch (Jan 1 1601).
func chromeCookieDate(timestampUTC int64) time.Time {
	if timestampUTC > windowsToUnixMicrosecondsOffset {
		timestampUTC -= windowsToUnixMicrosecondsOffset
	}

	return time.Unix(timestampUTC/1000000, (timestampUTC%1000000)*1000)
}
