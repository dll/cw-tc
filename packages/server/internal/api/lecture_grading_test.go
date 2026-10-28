package api

import "testing"

func TestGradeLectureAnswer(t *testing.T) {
	reference := "静态链接会把库代码复制进可执行文件，导致体积变大；动态链接在运行时共享同一份库代码，节省内存。"
	full, graded := gradeLectureAnswer("静态链接会把库代码复制进可执行文件，导致体积变大；动态链接在运行时共享同一份库代码，节省内存。", reference)
	if !graded || full < 90 {
		t.Fatalf("完全命中应接近满分: score=%d graded=%v", full, graded)
	}
	partial, graded := gradeLectureAnswer("静态链接会让程序体积变大，而动态链接在运行时共享库代码。", reference)
	if !graded || partial <= 0 || partial >= full {
		t.Fatalf("部分命中应得到中间分: score=%d graded=%v", partial, graded)
	}
	unrelated, graded := gradeLectureAnswer("今天天气不错，我想出去散步。", reference)
	if !graded || unrelated >= partial {
		t.Fatalf("无关答案应低于部分命中: score=%d graded=%v", unrelated, graded)
	}
	short, graded := gradeLectureAnswer("体积变大", reference)
	if !graded || short > 30 {
		t.Fatalf("过短答案应封顶 30 分: score=%d graded=%v", short, graded)
	}
	if _, graded := gradeLectureAnswer("任意作答", "略"); graded {
		t.Fatal("无有效参考答案时不应自动评分")
	}
}
