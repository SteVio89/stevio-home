package common

import (
	"net/url"
	"strconv"
)

// ParsePaginationParams reads `page` and `per_page` from the URL query.
// Empty values are returned as 0 (callers are expected to feed those into
// dbutil.ClampPagination, which substitutes defaults). Non-empty values that
// fail to parse return an error so the handler can respond 400 instead of
// silently coercing garbage to 0 (which used to surface as 500s deeper in
// the SQL layer).
func ParsePaginationParams(q url.Values) (page, perPage int, err error) {
	if raw := q.Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, err
		}
	}
	if raw := q.Get("per_page"); raw != "" {
		perPage, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, err
		}
	}
	return page, perPage, nil
}
