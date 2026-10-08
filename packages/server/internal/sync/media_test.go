package sync

import (
	"context"
	"testing"
)

func TestImportMediaAssets(t *testing.T) {
	database, cleanup := syncDB(t)
	defer cleanup()
	root := buildFixture(t)
	if err := Run(context.Background(), database, root); err != nil {
		t.Fatal(err)
	}
	entries := []MediaEntry{
		{StorageKey: "AI化教学创新03-CF/01_你好世界/video/讲解.mp4", Kind: "video", Size: 123, CourseCode: "03-CF", LectureNo: 1},
		{StorageKey: "AI化教学创新03-CF/01_你好世界/video/讲解.mp3", Kind: "audio", Size: 45, CourseCode: "03-CF", LectureNo: 1},
		// 讲次不存在 → 跳过
		{StorageKey: "AI化教学创新03-CF/99_不存在/video/x.mp4", Kind: "video", Size: 1, CourseCode: "03-CF", LectureNo: 99},
	}
	n, err := ImportMediaAssets(context.Background(), database, entries)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("应导入 2 条，实际 %d", n)
	}
	var count int
	_ = database.QueryRow(`SELECT COUNT(*) FROM asset WHERE kind IN ('video','audio')`).Scan(&count)
	if count != 2 {
		t.Fatalf("asset 应有 2 条媒体，实际 %d", count)
	}
	// 幂等：重复导入不重复建行（ON CONFLICT storage_key upsert）
	n2, err := ImportMediaAssets(context.Background(), database, []MediaEntry{
		{StorageKey: "AI化教学创新03-CF/01_你好世界/video/讲解.mp4", Kind: "video", Size: 999, CourseCode: "03-CF", LectureNo: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 1 {
		t.Fatalf("重复导入应仍返回 1 条，实际 %d", n2)
	}
	var size int64
	_ = database.QueryRow(`SELECT size_bytes FROM asset WHERE storage_key = 'AI化教学创新03-CF/01_你好世界/video/讲解.mp4'`).Scan(&size)
	if size != 999 {
		t.Fatalf("重复导入应覆盖 size 为 999，实际 %d", size)
	}
}
