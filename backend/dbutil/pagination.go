package dbutil

// ClampPagination normalises page/perPage values to safe defaults.
// page starts at 1; perPage is clamped to [1, 100] with a default of 20.
func ClampPagination(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}
