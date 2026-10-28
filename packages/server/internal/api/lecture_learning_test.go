package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cw-tc/server/internal/platform/db"
	"github.com/cw-tc/server/migrations"
	"github.com/gin-gonic/gin"
)

func learningFixture(t *testing.T) (*Server, *gin.Engine) {
	t.Helper()
	database, err := db.Open(context.Background(), db.DialectSQLite, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err = db.ApplyMigrations(context.Background(), database, migrations.FS, db.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`INSERT INTO student(id,student_no,name,phone,id_card_hash,id_card_cipher,agreement_versions,registration_review_status) VALUES(1,'S1','学员','13812345678',X'01',X'02','v1','approved');
	INSERT INTO staff(id,account,name,role,password_hash,created_source) VALUES(2,'teacher','教师','teacher','hash','seed');
	INSERT INTO major(id,name) VALUES(1,'样例专业');
	INSERT INTO course(id,course_code,source,name,line,major_id,status,price_cents,free_preview_lectures,teacher_id) VALUES(1,'C1','platform','课程','main',1,'published',1000,1,2);
	INSERT INTO lecture(id,course_id,no,title,is_preview) VALUES(1,1,1,'第一讲',0),(2,1,2,'第二讲',0);
	INSERT INTO lecture_question(id,lecture_id,source_key,prompt,reference_answer,evidence_ref) VALUES(1,1,'1','练习题','参考答案','讲义第一节');
	INSERT INTO question(id,course_id,qno,level,type,gradability,stem,score) VALUES(1,1,'Q1','L1','single','auto','这是一道用于检测题目泄漏的测评题干内容',1);`)
	if err != nil {
		t.Fatal(err)
	}
	server := NewWithDatabase(testConfig(t, true), database)
	r := server.Router()
	return server, r
}

func TestDiscussionBodyPrivacy(t *testing.T) {
	for _, body := range []string{"讨论这讲的网络结构", "联系我 13812345678", "样例 110101199001011234", "测评答案如下"} {
		ok := discussionBodyOK(body)
		if ok != (body == "讨论这讲的网络结构") {
			t.Fatalf("内容校验异常: %q: %v", body, ok)
		}
	}
}

func TestDiscussionLeakAndUnauthenticatedPractice(t *testing.T) {
	server, r := learningFixture(t)

	// 未登录访问讲次练习应被拒绝。
	if got := performJSON(t, r, http.MethodGet, "/api/v1/courses/C1/lectures/1/practice", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("未登录练习请求应 401，实际 %d: %s", got.Code, got.Body.String())
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if server.discussionExamLeak(ctx, 1, "普通讨论内容") {
		t.Fatal("普通讨论不应命中测评题干")
	}
	if !server.discussionExamLeak(ctx, 1, "这是一道用于检测题目泄漏的测评题干内容") {
		t.Fatal("命中测评题干时必须拦截")
	}
	if publicDiscussionName(12345) != "学员2345" {
		t.Fatalf("公开讨论名未脱敏")
	}
}
