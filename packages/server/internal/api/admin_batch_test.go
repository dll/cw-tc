package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAdminBatchPriceAndMajor(t *testing.T) {
	server, database, token, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id) VALUES('C2','platform','第二课','main',1,'published',2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO major(id,name) VALUES(2,'第二专业')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	preview := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/batch/courses",
		`{"action":"price","course_codes":["C1","C2"],"price_cents":2000,"preview":true}`, token)
	if preview.Code != http.StatusOK || !containsAll(preview.Body.String(), `"would_apply":true`) {
		t.Fatalf("批量预览失败: %d %s", preview.Code, preview.Body.String())
	}
	var payload struct {
		Data struct {
			BatchID string `json:"batch_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &payload)
	if payload.Data.BatchID == "" {
		t.Fatalf("缺少 batch_id: %s", preview.Body.String())
	}
	var price int64
	_ = database.QueryRow(`SELECT price_cents FROM course WHERE course_code='C1'`).Scan(&price)
	if price == 2000 {
		t.Fatal("预览不应修改数据")
	}
	exec := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/batch/courses",
		`{"action":"price","course_codes":["C1","C2"],"price_cents":2000,"reason":"统一调价","confirm":true}`, token)
	if exec.Code != http.StatusOK {
		t.Fatalf("批量调价失败: %d %s", exec.Code, exec.Body.String())
	}
	_ = database.QueryRow(`SELECT price_cents FROM course WHERE course_code='C1'`).Scan(&price)
	if price != 2000 {
		t.Fatalf("批量调价未生效: %d", price)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/batch/"+payload.Data.BatchID, "", token); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"preview":true`) {
		t.Fatalf("批量结果查询异常: %d", got.Code)
	}
	major := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/batch/courses",
		`{"action":"major","course_codes":["C1"],"major_id":2,"confirm":true}`, token)
	if major.Code != http.StatusOK {
		t.Fatalf("批量挂专业失败: %d %s", major.Code, major.Body.String())
	}
	for _, body := range []string{
		`{"action":"bad","course_codes":["C1"],"confirm":true}`,
		`{"action":"price","course_codes":["C1"],"price_cents":-1,"confirm":true}`,
		`{"action":"major","course_codes":["C1"],"major_id":0,"confirm":true}`,
		`{"action":"price","course_codes":["C1"],"price_cents":1,"confirm":false}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/admin/batch/courses", body, token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法批量操作应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/batch/999", "", token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在批量任务应 404: %d", got.Code)
	}
}
