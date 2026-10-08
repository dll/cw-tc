package exam

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAllocateCertNoSequential(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	first, err := AllocateCertNo(ctx, database, "C1", "L2")
	if err != nil {
		t.Fatal(err)
	}
	second, err := AllocateCertNo(ctx, database, "C1", "L2")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("取号不应重复: %s", first)
	}
	want := fmt.Sprintf("CW-CN-M%d%05d", certYear(time.Now().UTC()), 1)
	if first != want {
		t.Fatalf("首个编号应为 %s，实际 %s", want, first)
	}
}

// TestAllocateCertNoConcurrentUniqueness 并发取号：N 个 goroutine 同时取号，必须全部唯一。
func TestAllocateCertNoConcurrentUniqueness(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	const n = 100
	var wg sync.WaitGroup
	results := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = AllocateCertNo(context.Background(), database, "C1", "L1")
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("第 %d 次取号失败: %v", i, errs[i])
		}
		if seen[results[i]] {
			t.Fatalf("并发取号出现重复: %s", results[i])
		}
		seen[results[i]] = true
	}
	if len(seen) != n {
		t.Fatalf("应取到 %d 个唯一编号，实际 %d", n, len(seen))
	}
}

func TestIssueCertificateWritesUniqueRow(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	ctx := context.Background()
	certNo, err := IssueCertificate(ctx, database, CertIssueInput{
		StudentID: 1, CourseID: 1, CourseCode: "C1", Level: "L2", Score: 88, Band: "good",
	})
	if err != nil {
		t.Fatal(err)
	}
	if certNo == "" {
		t.Fatal("证书编号为空")
	}
	var issuedAt, expiresAt string
	if err := database.QueryRow(`SELECT issued_at, expires_at FROM certificate WHERE cert_no=$1`, certNo).Scan(&issuedAt, &expiresAt); err != nil {
		t.Fatal(err)
	}
	// 有效期 = 签发 + 3 年
	iss, _ := time.Parse(time.RFC3339, issuedAt)
	exp, _ := time.Parse(time.RFC3339, expiresAt)
	if got := exp.Year() - iss.Year(); got != 3 {
		t.Fatalf("有效期应为 3 年，实际差 %d 年", got)
	}
	// 同课同级当前只发一张（部分唯一索引）
	if _, err := IssueCertificate(ctx, database, CertIssueInput{
		StudentID: 1, CourseID: 1, CourseCode: "C1", Level: "L2", Score: 90, Band: "excellent",
	}); err == nil {
		t.Fatal("同课同级重复发证应被唯一索引拦截")
	}
}

func TestIssueCertificateInvalidInput(t *testing.T) {
	database := examDB(t)
	defer database.Close()
	if _, err := IssueCertificate(context.Background(), database, CertIssueInput{StudentID: 1, CourseID: 1, CourseCode: "C1", Level: "L9", Score: 1, Band: "pass"}); err != ErrInvalidCertLevel {
		t.Fatalf("非法档位应报 ErrInvalidCertLevel，实际 %v", err)
	}
	if _, err := IssueCertificate(context.Background(), database, CertIssueInput{}); err != ErrCertIssueInput {
		t.Fatalf("空入参应报 ErrCertIssueInput，实际 %v", err)
	}
}
