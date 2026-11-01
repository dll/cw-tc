package aicontent

import (
	"errors"
	"strings"
	"testing"
)

func sampleLectures() []Lecture {
	return []Lecture{{Title: "第一讲", ContentMD: strings.Repeat("分布式系统需要一致性与可用性。", 30)}}
}

func TestParseAndLabel(t *testing.T) {
	raw := "```json\n{\"lectures\":[{\"title\":\"第一讲\",\"content_md\":\"" + strings.Repeat("内容", 200) + "\"}]}\n```"
	lectures, err := ParseLectures(raw, 1)
	if err != nil || len(lectures) != 1 || lectures[0].Title != "第一讲" {
		t.Fatalf("解析失败: %+v %v", lectures, err)
	}
	labeled := WithAILabel(lectures)
	if !strings.HasPrefix(labeled[0].ContentMD, AILabelPrefix) {
		t.Fatal("必须写入显式 AI 标识")
	}
	if _, err := ParseLectures("not-json", 1); !errors.Is(err, ErrParse) {
		t.Fatalf("非法 JSON 应拒绝: %v", err)
	}
	if _, err := ParseLectures(raw, 2); !errors.Is(err, ErrCount) {
		t.Fatalf("数量不一致应拒绝: %v", err)
	}
}

func TestAutoCheck(t *testing.T) {
	lectures := sampleLectures()
	passed, report := AutoCheck(lectures, 1)
	if !passed || report["citations_ok"] != true {
		t.Fatalf("正常草稿应通过: %+v", report)
	}
	if passed, _ := AutoCheck(lectures, 0); passed {
		t.Fatal("无引用不得通过")
	}
	short := []Lecture{{Title: "太短", ContentMD: "一句话"}}
	if passed, _ := AutoCheck(short, 1); passed {
		t.Fatal("长度不足不得通过")
	}
	unsafe := sampleLectures()
	unsafe[0].ContentMD += "标准答案"
	if passed, _ := AutoCheck(unsafe, 1); passed {
		t.Fatal("含试题答案不得通过")
	}
	dup := append(sampleLectures(), sampleLectures()...)
	if passed, _ := AutoCheck(dup, 1); passed {
		t.Fatal("重复讲次不得通过")
	}
	if Hash(lectures) == Hash(WithAILabel(lectures)) {
		t.Fatal("内容变化后指纹必须变化")
	}
}
