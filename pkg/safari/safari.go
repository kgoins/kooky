package safari

// Read safari kooky.Cookie.binarycookies files.
// Thanks to https://github.com/as0ler/BinaryCookieReader

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	kooky "github.com/kgoins/kooky/pkg"
)

type fileHeader struct {
	Magic    [4]byte
	NumPages int32
}

type pageHeader struct {
	Header     [4]byte
	NumCookies int32
}

type cookieHeader struct {
	Size           int32
	Unknown1       int32
	Flags          int32
	Unknown2       int32
	URLOffset      int32
	NameOffset     int32
	PathOffset     int32
	ValueOffset    int32
	End            [8]byte
	ExpirationDate float64
	CreationDate   float64
}

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

// CookieReader implements kooky.KookyReader for the Safari browser
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

// ReadAllCookies reads all cookies from the input safari cookie database filepath.
func (reader CookieReader) ReadAllCookies(filename string) ([]*kooky.Cookie, error) {
	return reader.ReadCookies(filename, "", "", time.Time{})
}

// ReadCookies reads cookies from the input safari cookie database filepath, filtered by the input parameters.
func (reader CookieReader) ReadCookies(filename string, domainFilter string, nameFilter string, expireAfter time.Time) ([]*kooky.Cookie, error) {
	var allCookies []*kooky.Cookie

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var header fileHeader
	err = binary.Read(f, binary.BigEndian, &header)
	if err != nil {
		return nil, fmt.Errorf("error reading header: %v", err)
	}
	if string(header.Magic[:]) != "cook" {
		return nil, fmt.Errorf("expected first 4 bytes to be %q; got %q", "cook", string(header.Magic[:]))
	}

	pageSizes := make([]int32, header.NumPages)
	if err = binary.Read(f, binary.BigEndian, &pageSizes); err != nil {
		return nil, fmt.Errorf("error reading page sizes: %v", err)
	}

	for i, pageSize := range pageSizes {
		if allCookies, err = readPage(f, pageSize, allCookies); err != nil {
			return nil, fmt.Errorf("error reading page %d: %v", i, err)
		}
	}

	// TODO(zellyn): figure out how the checksum works.
	var checksum [8]byte
	err = binary.Read(f, binary.BigEndian, &checksum)
	if err != nil {
		return nil, fmt.Errorf("error reading checksum: %v", err)
	}

	// Filter cookies by specified filters.
	var cookies []*kooky.Cookie
	for _, cookie := range allCookies {
		if domainFilter != "" && domainFilter != cookie.Domain {
			continue
		}
		if nameFilter != "" && nameFilter != cookie.Name {
			continue
		}
		if !cookie.Expires.IsZero() && cookie.Expires.Before(expireAfter) {
			continue
		}

		cookies = append(cookies, cookie)
	}
	return cookies, nil
}

func readPage(f io.Reader, pageSize int32, cookies []*kooky.Cookie) ([]*kooky.Cookie, error) {
	bb := make([]byte, pageSize)
	if _, err := io.ReadFull(f, bb); err != nil {
		return nil, err
	}
	r := bytes.NewReader(bb)

	var header pageHeader
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("error reading header: %v", err)
	}
	want := [4]byte{0x00, 0x00, 0x01, 0x00}
	if header.Header != want {
		return nil, fmt.Errorf("expected first 4 bytes of page to be %v; got %v", want, header.Header)
	}

	cookieOffsets := make([]int32, header.NumCookies)
	if err := binary.Read(r, binary.LittleEndian, &cookieOffsets); err != nil {
		return nil, fmt.Errorf("error reading cookie offsets: %v", err)
	}

	for i, cookieOffset := range cookieOffsets {
		r.Seek(int64(cookieOffset), io.SeekStart)
		cookie, err := readCookie(r)
		if err != nil {
			return nil, fmt.Errorf("cookie %d: %v", i, err)
		}
		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

func readCookie(r io.ReadSeeker) (*kooky.Cookie, error) {
	start, _ := r.Seek(0, io.SeekCurrent)
	var ch cookieHeader
	if err := binary.Read(r, binary.LittleEndian, &ch); err != nil {
		return nil, err
	}

	expiry := safariCookieDate(ch.ExpirationDate)
	creation := safariCookieDate(ch.CreationDate)

	url, err := readString(r, "url", start, ch.URLOffset)
	if err != nil {
		return nil, err
	}
	name, err := readString(r, "name", start, ch.NameOffset)
	if err != nil {
		return nil, err
	}
	path, err := readString(r, "path", start, ch.PathOffset)
	if err != nil {
		return nil, err
	}
	value, err := readString(r, "value", start, ch.ValueOffset)
	if err != nil {
		return nil, err
	}

	cookie := &kooky.Cookie{}
	cookie.Expires = expiry
	cookie.Creation = creation
	cookie.Name = name
	cookie.Value = value
	cookie.Domain = url
	cookie.Path = path
	cookie.Secure = (ch.Flags & 1) > 0
	cookie.HttpOnly = (ch.Flags & 4) > 0

	return cookie, nil
}

func readString(r io.ReadSeeker, field string, start int64, offset int32) (string, error) {
	if _, err := r.Seek(start+int64(offset), io.SeekStart); err != nil {
		return "", fmt.Errorf("seeking for %q at offset %d", field, offset)
	}

	b := bufio.NewReader(r)
	value, err := b.ReadString(0)
	if err != nil {
		return "", fmt.Errorf("reading for %q at offset %d", field, offset)
	}

	return value[:len(value)-1], nil
}

// safariCookieDate converts double seconds to a time.Time object,
// accounting for the switch to Mac epoch (Jan 1 2001).
func safariCookieDate(floatSecs float64) time.Time {
	seconds, frac := math.Modf(floatSecs)
	return time.Unix(int64(seconds)+978307200, int64(frac*1000000000))
}
