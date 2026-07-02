package queries

import "github.com/SteVio89/stevio-home/dbutil"

func clampPagination(page, perPage int) (int, int) {
	return dbutil.ClampPagination(page, perPage)
}
