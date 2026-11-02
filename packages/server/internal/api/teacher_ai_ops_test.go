package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cw-tc/server/internal/staff"
)

func TestTeacherInsightsPlansAndCompare(t *testing.T) {
	server, database, _, _ := adminDetailServer(t)
	defer database.Close()
	if _, err := staff.Create(context.Background(), database, "t2", "教师二", staff.RoleTeacher, "Temp@123", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO course_insight(id,course_code,period_start,period_end,summary,signals,feedback_clusters,suggestions,unknowns,evidence_map,generated_at)
		VALUES(1,'C1','2026-10-24','2026-10-31','学习人数偏低','[]','[]','[{"problem":"例子不足"}]','[]','{"0":["stats.learning"]}','2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	r := server.Router()
	teacher := teacherToken(t, r, "t1")
	if list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/insights", "", teacher); list.Code != http.StatusOK || !containsAll(list.Body.String(), `"summary"`, `"verdict":"pending"`) {
		t.Fatalf("周报列表异常: %d %s", list.Code, list.Body.String())
	}
	if detail := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/insights/1", "", teacher); detail.Code != http.StatusOK || !containsAll(detail.Body.String(), `"evidence_map"`, `"suggestions"`) {
		t.Fatalf("周报详情异常: %d %s", detail.Code, detail.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/insights/1/verdict", `{"verdict":"rejected"}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("驳回缺理由应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/insights/1/verdict", `{"verdict":"accepted","note":"已排期"}`, teacher); got.Code != http.StatusOK {
		t.Fatalf("采纳失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/insights/1/verdict", `{"verdict":"rejected","note":"重复"}`, teacher); got.Code != http.StatusConflict {
		t.Fatalf("重复结论应 409，实际 %d", got.Code)
	}
	other := teacherToken(t, r, "t2")
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/insights/1", "", other); got.Code != http.StatusForbidden {
		t.Fatalf("非本课教师应 403，实际 %d", got.Code)
	}

	for _, body := range []string{
		`{"course_code":"C1","problem":"p","evidence":"","measure":"m","target_metric":"t"}`,
		`{"course_code":"C1","source":"bad","problem":"p","evidence":"e","measure":"m","target_metric":"t"}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/revision-plans", body, teacher); got.Code != http.StatusBadRequest {
			t.Fatalf("非法改版计划应 400: %s -> %d", body, got.Code)
		}
	}
	created := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/revision-plans",
		`{"course_code":"C1","source":"ai","insight_id":1,"problem":"例子不足","evidence":"周报建议 0","measure":"补充案例","priority":"P1","target_metric":"learning +20%"}`,
		teacher)
	if created.Code != http.StatusOK || !containsAll(created.Body.String(), `"id"`) {
		t.Fatalf("创建改版计划失败: %d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &payload)
	planID := payload.Data.ID
	if list := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/revision-plans?course_code=C1", "", teacher); list.Code != http.StatusOK || !containsAll(list.Body.String(), `"evidence":"周报建议 0"`) {
		t.Fatalf("改版计划列表异常: %d", list.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/revision-plans/"+itoaTest(planID),
		`{"status":"abandoned"}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("放弃缺原因应 400，实际 %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/revision-plans/"+itoaTest(planID),
		`{"status":"done"}`, teacher); got.Code != http.StatusOK {
		t.Fatalf("完成改版计划失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/revision-plans/"+itoaTest(planID),
		`{"status":"abandoned","abandon_reason":"优先级调整"}`, other); got.Code != http.StatusForbidden {
		t.Fatalf("非本课教师更新应 403，实际 %d", got.Code)
	}

	if _, err := database.Exec(`INSERT INTO course_revision(id,course_code,revision_no,content_version,changed_digest,published_at)
		VALUES(1,'C1',1,2,'补充案例','2026-10-31T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1','2026-10-20',10,1,5,2,1,1,1,1,5,0,'2026-10-20T00:00:00Z')`,
		`INSERT INTO course_stat_daily(course_code,stat_date,views_uv,favorites,learning,completed,examined,certified,orders,paid,unlock,rating_count,computed_at) VALUES('C1','2026-11-05',20,2,8,4,3,2,2,2,8,0,'2026-11-05T00:00:00Z')`,
	} {
		if _, err := database.Exec(row); err != nil {
			t.Fatal(err)
		}
	}
	compare := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/revisions/1/compare", "", teacher)
	if compare.Code != http.StatusOK || !containsAll(compare.Body.String(), `"before"`, `"after"`, `"learning":5`, `"learning":8`) {
		t.Fatalf("版本对比异常: %d %s", compare.Code, compare.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPost, "/api/v1/teacher/insights/1/verdict", `{"verdict":"bad"}`, teacher); got.Code != http.StatusBadRequest {
		t.Fatalf("非法周报结论应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/insights/999", "", teacher); got.Code != http.StatusNotFound {
		t.Fatalf("不存在周报应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/teacher/revision-plans/999", `{"status":"done"}`, teacher); got.Code != http.StatusNotFound {
		t.Fatalf("不存在改版计划应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/teacher/revisions/999/compare", "", teacher); got.Code != http.StatusNotFound {
		t.Fatalf("不存在版本应 404: %d", got.Code)
	}
}
