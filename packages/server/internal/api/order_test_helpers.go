package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func performJSONWithCookie(t *testing.T, r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "cw_tc_sid", Value: token})
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
