package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPublicSiteContentJobMapAndShowcase(t *testing.T) {
	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()

	// AI 公示缺失时公开概览 fail-closed。
	if got := performJSON(t, r, http.MethodGet, "/api/v1/public/overview", ""); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("缺 AI 公示应不可用: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content",
		`{"block":"filing_note","slot":1,"body":"deepseek-chat / 备案号 FIL-1","status":"published","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("发布无依据内容应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content",
		`{"block":"filing_note","slot":1,"body":"deepseek-chat / 备案号 FIL-1","evidence_ref":"docs/M4","status":"published","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("发布 AI 公示失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content",
		`{"block":"highlight","slot":1,"body":"真实项目案例","evidence_ref":"course:C1","status":"published","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("发布亮点失败: %d", got.Code)
	}
	overview := performJSON(t, r, http.MethodGet, "/api/v1/public/overview", "")
	if overview.Code != http.StatusOK || !containsAll(overview.Body.String(), `"filing_note"`, `"highlight"`) {
		t.Fatalf("公开概览异常: %d %s", overview.Code, overview.Body.String())
	}

	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/job-map",
		`{"course_code":"C1","cert_level":"L1","job_direction":"数据库工程师","skill_items":["SQL"],"lecture_refs":[],"evidence_ref":"证书:L1","status":"published","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("缺讲次引用发布应 400: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/job-map",
		`{"course_code":"C1","cert_level":"L1","job_direction":"数据库工程师","skill_items":["SQL"],"lecture_refs":[{"lecture_no":1}],"evidence_ref":"证书:L1","status":"published","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("发布就业映射失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/public/job-map", ""); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"job_direction":"数据库工程师"`, `"lecture_refs"`) {
		t.Fatalf("公开就业映射异常: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/job-map",
		`{"id":1,"course_code":"C1","cert_level":"L1","job_direction":"数据库工程师","skill_items":["SQL"],"lecture_refs":[{"lecture_no":1}],"evidence_ref":"证书:L1","status":"retired","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("下线就业映射失败: %d %s", got.Code, got.Body.String())
	}

	student := teacherToken(t, r, "t1") // 仅用于拿登录流程；下面创建学员会话
	_ = student
	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/showcase",
		`{"kind":"job_landed","nickname":"小王","summary":"拿到数据库岗位","job_direction":"数据库工程师","company_name":"样例公司","company_consent":false}`, session.Token); got.Code != http.StatusBadRequest {
		t.Fatalf("企业名未授权应 400: %d", got.Code)
	}
	created := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/showcase",
		`{"kind":"cert_earned","nickname":"小王","summary":"通过 L1 认证","cert_no":"CW-CN-A202600001","display_consent":true}`, session.Token)
	if created.Code != http.StatusOK || !containsAll(created.Body.String(), `"self_report"`) {
		t.Fatalf("提交成果失败: %d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &payload)
	if public := performJSON(t, r, http.MethodGet, "/api/v1/public/showcase", ""); public.Code != http.StatusOK || strings.Contains(public.Body.String(), `"小王"`) {
		t.Fatalf("未核验成果不得公开: %d %s", public.Code, public.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/showcase",
		`{"id":1,"action":"verify","note":"已核验证书","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("核验成果失败: %d %s", got.Code, got.Body.String())
	}
	public := performJSON(t, r, http.MethodGet, "/api/v1/public/showcase", "")
	if public.Code != http.StatusOK || !containsAll(public.Body.String(), `"nickname":"小王"`, `"cert_level":"L1"`) {
		t.Fatalf("公开成果异常: %d %s", public.Code, public.Body.String())
	}
	if strings.Contains(public.Body.String(), "student_id") || strings.Contains(public.Body.String(), "样例学员") {
		t.Fatalf("公开成果不得含真实身份: %s", public.Body.String())
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/showcase/"+itoaTest(payload.Data.ID)+"/revoke", `{}`, session.Token); got.Code != http.StatusOK {
		t.Fatalf("撤回成果失败: %d", got.Code)
	}
	if public := performJSON(t, r, http.MethodGet, "/api/v1/public/showcase", ""); strings.Contains(public.Body.String(), `"小王"`) {
		t.Fatalf("撤回后应立即下架: %s", public.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/showcase",
		`{"id":1,"action":"withdraw","note":"管理员下架","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("管理员下架成果失败: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/showcase",
		`{"id":1,"action":"bad","note":"x","confirm":true}`, adminToken); got.Code != http.StatusBadRequest {
		t.Fatalf("非法成果操作应 400: %d", got.Code)
	}
}

func TestPublicSiteValidationBranches(t *testing.T) {
	server, database, adminToken, _ := adminDetailServer(t)
	defer database.Close()
	r := server.Router()
	for _, body := range []string{
		`{"block":"bad","slot":1,"body":"x"}`,
		`{"block":"highlight","slot":1,"body":""}`,
		`{"block":"filing_note","slot":1,"body":"","evidence_ref":"e","status":"published","confirm":true}`,
		`{"block":"highlight","slot":1,"body":"x","evidence_ref":"e","status":"published","confirm":false}`,
		`{"block":"highlight","slot":1,"body":"x","evidence_ref":"e","status":"bad","confirm":true}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content", body, adminToken); got.Code != http.StatusBadRequest {
			t.Fatalf("非法内容应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content",
		`{"block":"highlight","slot":2,"body":"草稿内容","status":"draft"}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("保存草稿失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/content?status=draft", "", adminToken); got.Code != http.StatusOK || !containsAll(got.Body.String(), `"status":"draft"`) {
		t.Fatalf("内容列表异常: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/content",
		`{"block":"highlight","slot":2,"body":"草稿内容","status":"retired","confirm":true}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("下线内容块失败: %d %s", got.Code, got.Body.String())
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/job-map", "", adminToken); got.Code != http.StatusOK {
		t.Fatalf("就业映射管理员列表异常: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodGet, "/api/v1/admin/showcase", "", adminToken); got.Code != http.StatusOK {
		t.Fatalf("成果管理员列表异常: %d", got.Code)
	}

	for _, body := range []string{
		`{"course_code":"C1","cert_level":"L9","job_direction":"x","skill_items":["a"]}`,
		`{"course_code":"C1","cert_level":"L1","job_direction":"x","skill_items":[]}`,
		`{"course_code":"C1","cert_level":"L1","job_direction":"x","skill_items":["a"],"status":"bad"}`,
		`{"course_code":"C1","cert_level":"L1","job_direction":"x","skill_items":["a"],"status":"published","evidence_ref":"e","lecture_refs":[],"confirm":true}`,
		`{"course_code":"C1","cert_level":"L1","job_direction":"x","skill_items":["a"],"status":"published","evidence_ref":"e","lecture_refs":[1],"confirm":false}`,
	} {
		if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/job-map", body, adminToken); got.Code != http.StatusBadRequest {
			t.Fatalf("非法就业映射应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/job-map",
		`{"course_code":"C1","cert_level":"L1","job_direction":"数据库","skill_items":["SQL"],"lecture_refs":[1],"status":"draft"}`, adminToken); got.Code != http.StatusOK {
		t.Fatalf("保存就业映射草稿失败: %d %s", got.Code, got.Body.String())
	}

	session, err := server.identity.CreateSession(context.Background(), 1, timeNowUTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"kind":"bad","nickname":"小王","summary":"x"}`,
		`{"kind":"job_landed","nickname":"样例学员","summary":"x","job_direction":"x"}`,
		`{"kind":"contest_award","nickname":"小王","summary":"x","contest_name":"比赛"}`,
		`{"kind":"job_landed","nickname":"小王","summary":"x"}`,
		`{"kind":"cert_earned","nickname":"小王","summary":"x","cert_no":"NOPE"}`,
		`{"kind":"job_landed","nickname":"小王","summary":"联系 13812345678","job_direction":"x"}`,
	} {
		if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/showcase", body, session.Token); got.Code != http.StatusBadRequest {
			t.Fatalf("非法成果应 400: %s -> %d", body, got.Code)
		}
	}
	if got := performJSONWithCookie(t, r, http.MethodGet, "/api/v1/me/showcase", "", session.Token); got.Code != http.StatusOK {
		t.Fatalf("我的成果列表异常: %d", got.Code)
	}
	if got := performJSONWithCookie(t, r, http.MethodPost, "/api/v1/me/showcase/999/revoke", `{}`, session.Token); got.Code != http.StatusNotFound {
		t.Fatalf("不存在成果撤回应 404: %d", got.Code)
	}
	if got := performJSONWithBearer(t, r, http.MethodPatch, "/api/v1/admin/showcase", `{"id":999,"action":"verify","note":"x","confirm":true}`, adminToken); got.Code != http.StatusNotFound {
		t.Fatalf("不存在成果核验应 404: %d", got.Code)
	}
	if got := performJSON(t, r, http.MethodGet, "/api/v1/public/showcase?limit=0", ""); got.Code != http.StatusBadRequest {
		t.Fatalf("非法分页应 400: %d", got.Code)
	}
}
