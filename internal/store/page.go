package store

import "strings"

func NormalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

func fuzzyTokens(q string) []string {
	out := []string{}
	for _, t := range strings.Fields(strings.TrimSpace(q)) {
		if t == "" {
			continue
		}
		out = append(out, "%"+t+"%")
	}
	return out
}

func appendFuzzy(where string, args []any, q string, cols ...string) (string, []any) {
	if len(cols) == 0 {
		return where, args
	}
	for _, pat := range fuzzyTokens(q) {
		parts := make([]string, len(cols))
		for i, c := range cols {
			parts[i] = c + " LIKE ?"
			args = append(args, pat)
		}
		where += " AND (" + strings.Join(parts, " OR ") + ")"
	}
	return where, args
}
