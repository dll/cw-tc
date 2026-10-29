package course

import (
	"context"
	"testing"
)

func TestUnpublishAndRepublish(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,lecture_count,major_id,status,teacher_id,price_cents,cert_names,cert_pass_lines,content_version,published_version)
		VALUES('C1','platform','课程','main',5,1,'published',1,5000,'{"L1":"证书"}','{"L1":60}',3,2)`); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := UnpublishCourse(ctx, database, "C1", 1); err != nil {
		t.Fatal(err)
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='C1'`).Scan(&status)
	if status != "unpublished" {
		t.Fatalf("下架后状态=%s", status)
	}
	// 重新上架：published_version 对齐 content_version，并追加版本记录
	if err := PublishCourse(ctx, database, "C1", 1); err != nil {
		t.Fatalf("重新上架失败: %v", err)
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='C1'`).Scan(&status)
	if status != "published" {
		t.Fatalf("重新上架后状态=%s", status)
	}
	var publishedVersion int
	_ = database.QueryRow(`SELECT published_version FROM course WHERE course_code='C1'`).Scan(&publishedVersion)
	if publishedVersion != 3 {
		t.Fatalf("重新上架后 published_version=%d，期望 3", publishedVersion)
	}
	var revisions int
	_ = database.QueryRow(`SELECT COUNT(*) FROM course_revision WHERE course_code='C1' AND revision_no=1 AND content_version=3`).Scan(&revisions)
	if revisions != 1 {
		t.Fatalf("发布后应新增 1 条版本记录，实际 %d", revisions)
	}
	// 不存在 / 非法状态
	if err := UnpublishCourse(ctx, database, "NOPE", 1); err != ErrCourseNotFound {
		t.Fatalf("不存在课程应 %v，实际 %v", ErrCourseNotFound, err)
	}
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,cert_names,cert_pass_lines) VALUES('C2','platform','待审','main',1,'approved',1,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	if err := UnpublishCourse(ctx, database, "C2", 1); err != ErrInvalidTransition {
		t.Fatalf("approved 不能直接下架: %v", err)
	}
}

func TestPublishRequiresMajorAndCertConfig(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	// 缺专业
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,status,teacher_id,cert_names,cert_pass_lines) VALUES('NO-MAJOR','platform','缺专业','main','approved',1,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	if err := PublishCourse(ctx, database, "NO-MAJOR", 1); err != ErrMajorRequired {
		t.Fatalf("缺专业应 %v，实际 %v", ErrMajorRequired, err)
	}
	// 缺证书配置（platform）
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id) VALUES('NO-CERT','platform','缺证书','main',1,'approved',1)`); err != nil {
		t.Fatal(err)
	}
	if err := PublishCourse(ctx, database, "NO-CERT", 1); err != ErrCertConfigRequired {
		t.Fatalf("缺证书应 %v，实际 %v", ErrCertConfigRequired, err)
	}
	// proposed 不能直接上架
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,cert_names,cert_pass_lines) VALUES('DRAFT','platform','草稿','main',1,'proposed',1,'{"L1":"证书"}','{"L1":60}')`); err != nil {
		t.Fatal(err)
	}
	if err := PublishCourse(ctx, database, "DRAFT", 1); err != ErrInvalidTransition {
		t.Fatalf("proposed 不能直接上架: %v", err)
	}
}

func TestReviewCourseBindsDatabaseContentVersion(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,major_id,status,teacher_id,content_version) VALUES('C1','platform','课程','main',1,'submitted',1,7)`); err != nil {
		t.Fatal(err)
	}
	err := ReviewCourse(ctx, database, ReviewInput{
		CourseCode: "C1", ContentVersion: 1, ReviewerID: 1, ReviewerRole: "admin_ops",
		ScoreMaterial: 25, ScoreObjective: 15, ScoreCase: 15,
		ScoreLessonPlan: 15, ScoreSlides: 15, ScoreLab: 15, Verdict: "approved",
	})
	if err != nil {
		t.Fatal(err)
	}
	var version int
	if err := database.QueryRow(`SELECT content_version FROM course_review WHERE course_code='C1'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 7 {
		t.Fatalf("审核应绑定数据库内容版本 7，实际 %d", version)
	}
}
