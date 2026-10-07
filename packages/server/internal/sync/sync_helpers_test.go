package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCourseNameStripsWrapper(t *testing.T) {
	cases := map[string]string{
		"《C语言程序设计》分级测试题库":             "C语言程序设计",
		"《软件工程基础 · DevOps 新范式》分级测试题库": "软件工程基础 · DevOps 新范式",
		"《计算机网络》分级测试题库":               "计算机网络",
	}
	for in, want := range cases {
		if got := courseName(in); got != want {
			t.Fatalf("courseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildCertNamesAndPassLines(t *testing.T) {
	tb := &Testbank{
		CertNames: []string{"认证一", "认证二", "认证三"},
		Levels:    map[string]Level{"L1": {Pass: 60}, "L2": {Pass: 70}, "L3": {Pass: 80}},
	}
	names := buildCertNames(tb)
	if !containsAll(names, `"L1":"认证一"`, `"L3":"认证三"`) {
		t.Fatalf("证书名 JSON 错误: %s", names)
	}
	lines := buildCertPassLines(tb)
	if !containsAll(lines, `"L1":60`, `"L3":80`) {
		t.Fatalf("合格线 JSON 错误: %s", lines)
	}
}

func TestLoadOverviewErrors(t *testing.T) {
	if _, err := LoadOverview(t.TempDir()); err == nil {
		t.Fatal("缺失总览文件应报错")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, overviewFile), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOverview(root); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

func TestPruneFunctionsEmptyKeep(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	if err := pruneUpstreamCourses(context.Background(), database, nil); err != nil {
		t.Fatalf("空 keep 清空上游课程失败: %v", err)
	}
	if err := pruneUpstreamLectures(context.Background(), database, 1, nil); err != nil {
		t.Fatalf("空 keep 清空上游讲次失败: %v", err)
	}
}

func TestRunWithNilDatabase(t *testing.T) {
	if err := Run(context.Background(), nil, t.TempDir()); err == nil {
		t.Fatal("nil 数据库应报错")
	}
}

func containsAll(value string, parts ...string) bool {
	for _, p := range parts {
		if !contains(value, p) {
			return false
		}
	}
	return true
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
