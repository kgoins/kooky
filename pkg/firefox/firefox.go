package firefox

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sqlite/sqlite3"
	kooky "github.com/kgoins/kooky/pkg"
)

var cookiePathMap kooky.DefaultPathMap
var installLocationPathMap kooky.DefaultPathMap

func init() {
	cookiePathMap = kooky.NewDefaultPathMap()
	cookiePathMap.Add("windows", `AppData\Roaming\Mozilla\Firefox\Profiles\`)
	cookiePathMap.Add("darwin", "Library/Application Support/Firefox/Profiles/")
	cookiePathMap.Add("linux", ".mozilla/firefox/")

	installLocationPathMap = kooky.NewDefaultPathMap()
	installLocationPathMap.Add("windows", `C:\Program Files\Mozilla Firefox\firefox.exe`)
	installLocationPathMap.Add("darwin", "/Applications/Firefox.app/Contents/MacOS/firefox")
	installLocationPathMap.Add("linux", "/usr/bin/firefox")
}

// CookieReader implements kooky.KookyReader for the Firefox browser
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

func getDefaultProfile(profileDirPath string) (string, error) {
	profileDir, err := ioutil.ReadDir(profileDirPath)
	if err != nil {
		return "", err
	}

	for _, entry := range profileDir {
		if entry.IsDir() {
			if strings.Contains(entry.Name(), ".default") {
				return entry.Name(), nil
			}
		}
	}

	return "", errors.New("Unable to locate default profile")
}

// GetDefaultCookieFilePath returns the absolute filepath for the file used to store cookies on the current OS.
func (reader CookieReader) GetDefaultCookieFilePath(operatingSystem string) (string, error) {
	path, found := reader.cookiePathMap.Get(operatingSystem)
	if !found {
		return "", errors.New("Unsupported operating system")
	}

	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	profileDirPath := filepath.Join(currentUser.HomeDir, path)
	defaultProfile, err := getDefaultProfile(profileDirPath)
	if err != nil {
		return "", err
	}

	return filepath.Join(profileDirPath, defaultProfile, "cookies.sqlite"), nil
}

// ReadCookies reads cookies from the input firefox sqlite database filepath, filtered by the input parameters.
func (reader CookieReader) ReadCookies(filename string, domainFilter string, nameFilter string, expireAfter time.Time) ([]*kooky.Cookie, error) {
	return reader.ReadAllCookies(filename)
}

// ReadAllCookies reads all cookies from the input firefox sqlite database filepath.
func (reader CookieReader) ReadAllCookies(filename string) ([]*kooky.Cookie, error) {
	var cookies []*kooky.Cookie
	db, err := sqlite3.Open(filename)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.VisitTableRecords("moz_cookies", func(rowId *int64, rec sqlite3.Record) error {
		if lRec := len(rec.Values); lRec != 13 && lRec != 14 {
			return fmt.Errorf("got %d columns, but expected 13 or 14", lRec)
		}

		cookie := kooky.Cookie{}
		var ok bool

		// Name
		cookie.Name, ok = rec.Values[3].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Name %v", rec.Values[3])
		}

		// Value
		cookie.Value, ok = rec.Values[4].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Value %v", rec.Values[4])
		}

		// Domain
		cookie.Domain, ok = rec.Values[1].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Domain %v", rec.Values[1])
		}

		// Path
		cookie.Path, ok = rec.Values[6].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Path %v", rec.Values[6])
		}

		// Expires
		if int32Value, ok := rec.Values[7].(int32); ok {
			cookie.Expires = time.Unix(int64(int32Value), 0)
		} else if uint64Value, ok := rec.Values[7].(uint64); ok {
			cookie.Expires = time.Unix(int64(uint64Value), 0)
		} else {
			return fmt.Errorf("got unexpected value for Expires %v (type %T)", rec.Values[7], rec.Values[7])
		}

		// Creation
		int64Value, ok := rec.Values[9].(int64)
		if !ok {
			return fmt.Errorf("got unexpected value for Creation %v (type %T)", rec.Values[9], rec.Values[9])
		}
		cookie.Creation = time.Unix(int64Value/1e6, 0) // drop nanoseconds

		// Secure
		intValue, ok := rec.Values[10].(int)
		if !ok {
			return fmt.Errorf("got unexpected value for Secure %v", rec.Values[10])
		}
		cookie.Secure = intValue > 0

		// HttpOnly
		intValue, ok = rec.Values[11].(int)
		if !ok {
			return fmt.Errorf("got unexpected value for HttpOnly %v", rec.Values[11])
		}
		cookie.HttpOnly = intValue > 0

		cookies = append(cookies, &cookie)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return cookies, nil
}
