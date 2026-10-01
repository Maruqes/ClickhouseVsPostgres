package database

import (
	"fmt"
	"strings"
)

// Dialect configuration is consumed by the same SQL execution and seed lifecycle.
type dialect struct {
	driver        string
	schema        []string
	series        string
	timestamp     string
	deleteTail    string
	querySettings string
	prepare       []string
	finish        []string
	numbered      bool
}

func (d dialect) bind(query string) string {
	if !d.numbered {
		return query
	}
	n := 0
	return replaceParams(query, &n)
}

func replaceParams(query string, n *int) string {
	var result strings.Builder
	for _, part := range strings.Split(query, "?") {
		if *n > 0 {
			fmt.Fprintf(&result, "$%d", *n)
		}
		result.WriteString(part)
		*n++
	}
	return result.String()
}

func (d dialect) source(start, count int64) string {
	end := count
	if d.numbered {
		end = start + count - 1
	}
	return fmt.Sprintf(d.series, start, end)
}
