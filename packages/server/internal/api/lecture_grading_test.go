package api

import "testing"

func TestNormalizeAndTerms(t *testing.T) {
	if got := normalizeForGrade("TCP, 三次握手! 2026"); got != "tcp三次握手2026" {
		t.Fatalf("normalizeForGrade=%q", got)
	}
	terms := significantTerms("TCP uses a three way handshake; sequence numbers 1234")
	for _, want := range []string{"tcp", "uses", "three", "handshake", "sequence", "numbers"} {
		if _, ok := terms[want]; !ok {
			t.Errorf("缺少英文关键词 %q: %#v", want, terms)
		}
	}
	cn := significantTerms("静态链接复制库代码，动态链接共享库代码")
	if _, ok := cn["静态"]; !ok {
		t.Errorf("缺少中文二元组: %#v", cn)
	}
	if _, ok := cn["因为"]; ok {
		t.Error("停用词不应进入关键词")
	}
}

func TestGradeLectureAnswerEdges(t *testing.T) {
	reference := "TCP 三次握手用于同步初始序号，避免历史连接请求造成误建。"
	if score, graded := gradeLectureAnswer("", reference); !graded || score != 0 {
		t.Fatalf("空答案应为 0 分: score=%d graded=%v", score, graded)
	}
	if _, graded := gradeLectureAnswer("任意", "略"); graded {
		t.Fatal("关键词过少时不应评分")
	}
	english := "TCP uses a three way handshake to synchronize initial sequence numbers"
	if score, graded := gradeLectureAnswer(english, reference+" TCP uses a three way handshake to synchronize initial sequence numbers"); !graded || score == 0 {
		t.Fatalf("英文关键词应参与评分: score=%d graded=%v", score, graded)
	}
}

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
