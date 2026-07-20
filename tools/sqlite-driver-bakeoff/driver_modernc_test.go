//go:build modernc

package sqlitedriverbakeoff

import (
	"net/url"

	_ "modernc.org/sqlite"
)

const benchmarkDriverName = "sqlite"

func benchmarkDriverDSN(path string, readOnly bool) string {
	dsn := &url.URL{Scheme: "file", Path: path}
	query := dsn.Query()
	query.Add("_pragma", "busy_timeout(5000)")
	if readOnly {
		query.Set("mode", "ro")
		query.Add("_pragma", "query_only(ON)")
	} else {
		query.Add("_pragma", "foreign_keys(ON)")
		query.Add("_pragma", "journal_mode(WAL)")
		query.Add("_pragma", "synchronous(FULL)")
	}
	dsn.RawQuery = query.Encode()
	return dsn.String()
}
