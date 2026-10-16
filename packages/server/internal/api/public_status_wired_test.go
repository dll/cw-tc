package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/internal/redact"
	"github.com/cw-tc/server/migrations"
)

// TestPublicStatusAggregatesFromDatabase 校验数据源接入后 /public/status
// 返回真实聚合计数（status=available）且**不含任何个人信息**。
func TestPublicStatusAggregatesFromDatabase(t *testing.T) {
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`INSERT INTO major(id,name) VALUES(1,'样例专业')`,
		`INSERT INTO staff(account,name,role,password_hash,created_source) VALUES('sample','样例教师','teacher','x','seed')`,
		// 样例学员（测试文件按铁律放行使用固定样例号）。
		`INSERT INTO student(id,student_no,name,phone,id_card_hash,id_card_cipher,agreement_versions)
			VALUES(1,'S001','样例学员','13812345678',X'00',X'00','["v1"]')`,
		`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,price_cents)
			VALUES('PUB','platform','已上架课程','main',1,'published',1,1000),
			      ('BLD','platform','在建课程','main',1,'building',1,2000)`,
		`INSERT INTO certificate(cert_no,student_id,course_id,level,score,band,issued_at,expires_at,status)
			VALUES('C-1',1,1,'L1',90,'优秀','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','valid')`,
		`INSERT INTO certificate(cert_no,student_id,course_id,level,score,band,issued_at,expires_at,status,revoked_reason,revoked_at)
			VALUES('C-2',1,2,'L2',80,'合格','2026-01-01T00:00:00Z','2029-01-01T00:00:00Z','revoked','admin','2026-02-01T00:00:00Z')`,
	}
	for _, s := range stmts {
		if _, err := database.Exec(s); err != nil {
			t.Fatalf("seed 失败: %v\n%s", err, s)
		}
	}

	r := NewWithDatabase(testConfig(t, true), database).Router()
	w, body := doGET(t, r, "/api/v1/public/status")
	if w.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", w.Code)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 缺失: %v", body)
	}
	if data["status"] != "available" || data["dataSource"] != "database" {
		t.Fatalf("接入数据源后应为 available/database，实际 %v/%v", data["status"], data["dataSource"])
	}
	courses := data["courses"].(map[string]any)
	if courses["total"] != float64(2) || courses["published"] != float64(1) {
		t.Fatalf("courses 计数错误: %v", courses)
	}
	certs := data["certificates"].(map[string]any)
	if certs["issued"] != float64(2) || certs["revoked"] != float64(1) {
		t.Fatalf("certificates 计数错误: %v", certs)
	}
	if learners := data["learners"].(map[string]any); learners["registered"] != float64(1) {
		t.Fatalf("learners 计数错误: %v", learners)
	}

	raw := w.Body.String()
	for _, forbidden := range []string{"id_card", "idCard", "phone", "13812345678", "样例学员", "studentNo"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("公开状态响应不得包含 %q: %s", forbidden, raw)
		}
	}
	if redact.ContainsIDCard(raw) || redact.ContainsPhone(raw) {
		t.Errorf("公开状态响应含身份证号或手机号: %s", raw)
	}
}
