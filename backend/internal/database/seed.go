package database

import (
	"fmt"
	"strings"
)

// Relative weights retain larger country populations while extending coverage
// to 50 countries. The original eleven weights total 100; 39 additions have one
// unit each. These synthetic weights do not model real national populations.
var seedCountries = []struct {
	code   string
	weight int
}{
	{"US", 30}, {"BR", 15}, {"DE", 12}, {"IN", 10}, {"GB", 8}, {"FR", 7},
	{"JP", 6}, {"AU", 5}, {"CA", 4}, {"PT", 2}, {"ZA", 1},
	{"ES", 1}, {"IT", 1}, {"NL", 1}, {"BE", 1}, {"CH", 1}, {"AT", 1},
	{"SE", 1}, {"NO", 1}, {"DK", 1}, {"FI", 1}, {"PL", 1}, {"CZ", 1},
	{"RO", 1}, {"GR", 1}, {"TR", 1}, {"UA", 1}, {"RU", 1}, {"CN", 1},
	{"KR", 1}, {"ID", 1}, {"TH", 1}, {"VN", 1}, {"MY", 1}, {"PH", 1},
	{"PK", 1}, {"BD", 1}, {"SA", 1}, {"AE", 1}, {"IL", 1}, {"EG", 1},
	{"MA", 1}, {"NG", 1}, {"KE", 1}, {"MX", 1}, {"AR", 1}, {"CL", 1},
	{"CO", 1}, {"PE", 1}, {"NZ", 1},
}

func seedCountrySQL() string {
	weight := 0
	for _, country := range seedCountries {
		weight += country.weight
	}
	var query strings.Builder
	query.WriteString("CASE")
	limit := 0
	for _, country := range seedCountries {
		limit += country.weight
		fmt.Fprintf(&query, " WHEN MOD(MOD(id - 1, 100000) * 37, %d) < %d THEN '%s'", weight, limit, country.code)
	}
	query.WriteString(" END")
	return query.String()
}
