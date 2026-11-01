package api

import (
	"net/http"
	"testing"
)

func TestAdminExamAnomalyReview(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	for _, kind := range []string{"blur", "too_fast"} {
		if _, err := database.Exec(`INSERT INTO exam_anomaly(exam_id,kind,detail,at,verdict) VALUES(1,$1,'{}','2026-10-31T02:00:00Z','pending')`, kind); err != nil {
			t.Fatal(err)
		}
	}
	list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/exams/1/anomalies", "", token)
	if list.Code != http.StatusOK || !containsAll(list.Body.String(), `"kind":"blur"`, `"kind":"too_fast"`, `"verdict":"pending"`) {
		t.Fatalf("异常列表异常: %d %s", list.Code, list.Body.String())
	}
	for _, body := range []string{
		`{"verdict":"pending","reason":"x","confirm":true}`,
		`{"verdict":"kept","reason":"","confirm":true}`,
		`{"verdict":"kept","reason":"x","confirm":false}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exams/1/verdict", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法结论应 400: %s -> %d", body, got.Code)
		}
	}
	var firstID int64
	if err := database.QueryRow(`SELECT id FROM exam_anomaly ORDER BY id LIMIT 1`).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	kept := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exams/1/verdict",
		`{"verdict":"kept","reason":"切屏为误触，维持成绩","anomaly_id":`+itoaTest(firstID)+`,"confirm":true}`, token)
	if kept.Code != http.StatusOK || !containsAll(kept.Body.String(), `"updated":1`) {
		t.Fatalf("单条复核失败: %d %s", kept.Code, kept.Body.String())
	}
	rest := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exams/1/verdict",
		`{"verdict":"manual_check","reason":"转人工核验设备","confirm":true}`, token)
	if rest.Code != http.StatusOK || !containsAll(rest.Body.String(), `"updated":1`) {
		t.Fatalf("批量复核失败: %d %s", rest.Code, rest.Body.String())
	}
	if again := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/exams/1/verdict",
		`{"verdict":"invalid","reason":"重复","confirm":true}`, token); again.Code != http.StatusConflict {
		t.Fatalf("无待复核记录应 409，实际 %d", again.Code)
	}
	var audits int
	if err := database.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='exam_anomaly.verdict'`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("异常复核必须写审计: %d %v", audits, err)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/exams/999/anomalies", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在测评应 404，实际 %d", got.Code)
	}
}
