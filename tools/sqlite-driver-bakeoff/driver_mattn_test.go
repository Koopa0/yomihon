//go:build mattn

package sqlitedriverbakeoff

import (
	"net/url"

	_ "github.com/mattn/go-sqlite3"
)

const benchmarkDriverName = "sqlite3"

func benchmarkDriverDSN(path string, readOnly bool) string {
	dsn := &url.URL{Scheme: "file", Path: path}
	query := dsn.Query()
	query.Set("_busy_timeout", "5000")
	if readOnly {
		query.Set("mode", "ro")
		query.Set("_query_only", "on")
	} else {
		query.Set("_foreign_keys", "on")
		query.Set("_journal_mode", "WAL")
		query.Set("_synchronous", "FULL")
	}
	dsn.RawQuery = query.Encode()
	return dsn.String()
}
