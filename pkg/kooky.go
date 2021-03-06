package kooky

import (
	"net/http"
	"time"
)

// TODO(zellyn): figure out what to do with quoted values, like the "bcookie" cookie
// from slideshare.net

// Cookie is the struct returned by functions in this package. Similar
// to http.Cookie, but just a dumb struct.
type Cookie struct {
	Domain   string
	Name     string
	Path     string
	Expires  time.Time
	Secure   bool
	HttpOnly bool
	Creation time.Time
	Value    string
}

// HttpCookie returns an http.Cookie equivalent to this Cookie.
func (c Cookie) HttpCookie() http.Cookie {
	hc := http.Cookie{}
	hc.Domain = c.Domain
	hc.Name = c.Name
	hc.Path = c.Path
	hc.Expires = c.Expires
	hc.Secure = c.Secure
	hc.HttpOnly = c.HttpOnly
	hc.Value = c.Value

	return hc
}

// FindCookie returns a cookie matching the input domain and name from a list of Cookies
func FindCookie(domain string, name string, cookies []*Cookie) *Cookie {
	for _, cookie := range cookies {
		if cookie.Domain == domain && cookie.Name == name {
			return cookie
		}
	}

	return nil
}
