package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cw-tc/server/internal/identity"
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
	INSERT INTO lecture_question(id,lecture_id,source_key,prompt,reference_answer,evidence_ref) VALUES(1,1,'1','练习题','静态链接会把库代码复制进可执行文件，动态链接在运行时共享同一份库代码。','讲义第一节');
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

func TestLecturePracticeAndDiscussionFlow(t *testing.T) {
	server, _ := learningFixture(t)
	r := gin.New()
	student := func(c *gin.Context) {
		c.Set(studentContextKey, identity.Student{ID: 1})
		c.Next()
	}
	r.GET("/courses/:courseCode/lectures/:no/practice", student, server.listLecturePractice)
	r.POST("/courses/:courseCode/lectures/:no/practice/:questionID/answers", student, server.submitLectureAnswer)
	r.GET("/courses/:courseCode/lectures/:no/topics", server.listLectureTopics)
	r.POST("/courses/:courseCode/lectures/:no/topics", student, server.createLectureTopic)
	r.GET("/courses/:courseCode/lectures/:no/topics/:topicID/replies", server.listLectureReplies)
	r.POST("/courses/:courseCode/lectures/:no/topics/:topicID/replies", student, server.createLectureReply)
	r.POST("/courses/:courseCode/lectures/:no/topics/:topicID/like", student, server.likeLectureTopic)
	r.DELETE("/courses/:courseCode/lectures/:no/topics/:topicID/like", student, server.unlikeLectureTopic)
	if got := performJSON(t, r, http.MethodGet, "/courses/C1/lectures/2/practice", ""); got.Code != http.StatusForbidden {
		t.Fatalf("未购课不可访问非预览讲次练习: %d %s", got.Code, got.Body.String())
	}
	practice := performJSON(t, r, http.MethodGet, "/courses/C1/lectures/1/practice", "")
	if practice.Code != http.StatusOK || !strings.Contains(practice.Body.String(), "讲义第一节") || strings.Contains(practice.Body.String(), "静态链接") {
		t.Fatalf("练习列表必须只返回题目和依据: %d %s", practice.Code, practice.Body.String())
	}
	answer := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/practice/1/answers", "{\"answer\":\"静态链接把库代码复制进可执行文件，动态链接运行时共享同一份库代码\"}")
	if answer.Code != http.StatusOK || !strings.Contains(answer.Body.String(), "静态链接会把库代码复制进可执行文件") || !strings.Contains(answer.Body.String(), `"graded":true`) || !strings.Contains(answer.Body.String(), `"score"`) {
		t.Fatalf("提交练习后应返回自动评分与参考答案: %d %s", answer.Code, answer.Body.String())
	}
	for _, tc := range []struct{ path, body string }{
		{"/courses/C1/lectures/1/practice/0/answers", "{\"answer\":\"有效答案\"}"},
		{"/courses/C1/lectures/1/practice/1/answers", "{\"answer\":\" \"}"},
		{"/courses/C1/lectures/1/practice/999/answers", "{\"answer\":\"有效答案\"}"},
	} {
		if got := performJSON(t, r, http.MethodPost, tc.path, tc.body); got.Code < 400 {
			t.Errorf("非法练习提交应失败: %s => %d %s", tc.path, got.Code, got.Body.String())
		}
	}

	created := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/topics", "{\"body\":\"讨论本讲的网络结构\"}")
	if created.Code != http.StatusOK {
		t.Fatalf("发布讨论失败: %d %s", created.Code, created.Body.String())
	}
	for _, body := range []string{
		"{\"body\":\"这是一道用于检测题目泄漏的测评题干内容\"}",
		"{\"body\":\"联系我 13812345678\"}",
	} {
		if got := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/topics", body); got.Code != http.StatusBadRequest {
			t.Errorf("违规讨论应拒绝: %d %s", got.Code, got.Body.String())
		}
	}
	list := performJSON(t, r, http.MethodGet, "/courses/C1/lectures/1/topics", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "\"author\":\"学员1\"") {
		t.Fatalf("讨论列表未脱敏显示发布者: %d %s", list.Code, list.Body.String())
	}
	reply := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/topics/1/replies", "{\"body\":\"我也有类似学习体会\"}")
	if reply.Code != http.StatusOK {
		t.Fatalf("发布回复失败: %d %s", reply.Code, reply.Body.String())
	}
	replies := performJSON(t, r, http.MethodGet, "/courses/C1/lectures/1/topics/1/replies", "")
	if replies.Code != http.StatusOK || !strings.Contains(replies.Body.String(), "学员1") {
		t.Fatalf("回复列表失败: %d %s", replies.Code, replies.Body.String())
	}
	if got := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/topics/999/replies", "{\"body\":\"有效回复内容\"}"); got.Code != http.StatusNotFound {
		t.Fatalf("不存在的讨论应返回 404: %d %s", got.Code, got.Body.String())
	}
	for i := 0; i < 2; i++ {
		if got := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/1/topics/1/like", ""); got.Code != http.StatusOK {
			t.Fatalf("点赞失败: %d %s", got.Code, got.Body.String())
		}
	}
	var likes int
	if err := server.dbConn.QueryRow("SELECT like_count FROM lecture_topic WHERE id=1").Scan(&likes); err != nil || likes != 1 {
		t.Fatalf("重复点赞不能重复计数: likes=%d err=%v", likes, err)
	}
	if got := performJSON(t, r, http.MethodDelete, "/courses/C1/lectures/1/topics/1/like", ""); got.Code != http.StatusOK {
		t.Fatalf("取消点赞失败: %d %s", got.Code, got.Body.String())
	}
	if err := server.dbConn.QueryRow("SELECT like_count FROM lecture_topic WHERE id=1").Scan(&likes); err != nil || likes != 0 {
		t.Fatalf("取消点赞后计数错误: likes=%d err=%v", likes, err)
	}
	if got := performJSON(t, r, http.MethodPost, "/courses/C1/lectures/2/topics", `{"body":"未购课学员也可发布讨论"}`); got.Code != http.StatusOK {
		t.Fatalf("登录学员应可发布讲次讨论（购课校验仅针对练习）: %d %s", got.Code, got.Body.String())
	}
	for _, path := range []string{"/courses/C1/lectures/x/practice", "/courses/missing/lectures/1/topics"} {
		if got := performJSON(t, r, http.MethodGet, path, ""); got.Code < 400 {
			t.Errorf("无效讲次应失败: %s => %d", path, got.Code)
		}
	}
}
