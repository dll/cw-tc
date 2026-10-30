package api

import (
	"net/http"
	"testing"
)

func TestCourseListSortAndLineFilters(t *testing.T) {
	s, _, cleanup := orderLifecycleServer(t)
	defer cleanup()
	r := s.Router()
	for _, path := range []string{
		"/api/v1/courses?sort=newest",
		"/api/v1/courses?sort=price_asc",
		"/api/v1/courses?sort=price_desc",
		"/api/v1/courses?sort=popular",
		"/api/v1/courses?line=main",
		"/api/v1/courses?line=aux",
	} {
		if got := performJSON(t, r, http.MethodGet, path, ""); got.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, got.Code, got.Body.String())
		}
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/courses?sort=bad", ""); got.Code != http.StatusBadRequest {
		t.Fatalf("非法排序应 400: %d", got.Code)
	}
}
