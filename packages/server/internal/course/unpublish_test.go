package course

import (
	"context"
	"testing"
)

func TestUnpublishAndRepublish(t *testing.T) {
	database := courseDB(t)
	defer database.Close()
	ctx := context.Background()
	if _, err := database.Exec(`INSERT INTO course(course_code,source,name,line,lecture_count,major_id,status,teacher_id,price_cents,cert_names,cert_pass_lines)
		VALUES('C1','platform','课程','main',5,1,'published',1,5000,'{"L1":"证书"}','{"L1":60}')`); err != nil {
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
	// 重新上架
	if err := PublishCourse(ctx, database, "C1", 1); err != nil {
		t.Fatalf("重新上架失败: %v", err)
	}
	_ = database.QueryRow(`SELECT status FROM course WHERE course_code='C1'`).Scan(&status)
	if status != "published" {
		t.Fatalf("重新上架后状态=%s", status)
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
