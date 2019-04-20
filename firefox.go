package kooky

import (
	"fmt"
	"time"

	"github.com/go-sqlite/sqlite3"
)

func ReadFirefoxCookies(filename string) ([]*Cookie, error) {
	var cookies []*Cookie
	db, err := sqlite3.Open(filename)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.VisitTableRecords("moz_cookies", func(rowId *int64, rec sqlite3.Record) error {
		if lRec := len(rec.Values); lRec != 13 && lRec != 14 {
			return fmt.Errorf("got %d columns, but expected 13 or 14")
		}

		cookie := Cookie{}

		// Name
		v, ok := rec.Values[3].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Name %v", rec.Values[3])
		}
		cookie.Name = v

		// Value
		v, ok = rec.Values[4].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Value %v", rec.Values[4])
		}
		cookie.Value = v

		// Domain
		v, ok = rec.Values[1].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Domain %v", rec.Values[1])
		}
		cookie.Domain = v

		// Path
		v, ok = rec.Values[6].(string)
		if !ok {
			return fmt.Errorf("got unexpected value for Path %v", rec.Values[6])
		}
		cookie.Path = v

		// Expires
		if v2, ok := rec.Values[7].(int32); ok {
		  cookie.Expires = time.Unix(int64(v2), 0)
		} else if v3, ok := rec.Values[7].(uint64); ok {
		  cookie.Expires = time.Unix(int64(v3), 0)
    } else {
			return fmt.Errorf("got unexpected value for Expires %v (type %T)", rec.Values[7], rec.Values[7])
		}

		// Creation
		v3, ok := rec.Values[9].(int64)
		if !ok {
			return fmt.Errorf("got unexpected value for Creation %v (type %T)", rec.Values[9], rec.Values[9])
		}
		cookie.Creation = time.Unix(v3/1e6, 0) // drop nanoseconds

		// Secure
		v4, ok := rec.Values[10].(int)
		if !ok {
			return fmt.Errorf("got unexpected value for Secure %v", rec.Values[10])
		}
		cookie.Secure = v4 > 0

		// HttpOnly
		v4, ok = rec.Values[11].(int)
		if !ok {
			return fmt.Errorf("got unexpected value for HttpOnly %v", rec.Values[11])
		}
		cookie.HttpOnly = v4 > 0

		cookies = append(cookies, &cookie)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return cookies, nil
}
