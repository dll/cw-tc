package sync

import (
	"strings"
	"testing"
)

func TestParseLecturePromptsWithSolutions(t *testing.T) {
	body := "# 思考题\n## ① 为什么分层？\n提示文字\n**解答**\n因为隔离变化。\n---\n## ② 下一题？\n**答案**\n第二题解答。\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if items[0].prompt == "" || items[0].answer == "" || items[1].answer == "" {
		t.Fatalf("提取结果不完整: %#v", items)
	}
}

func TestParseLecturePromptsWithoutSolutions(t *testing.T) {
	body := "## 基础题\n**1. 哪些操作会引发系统调用？**\n- 读取文件\n---\n**2. 为什么？**\n请说明理由\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if items[0].answer != "" || items[1].answer != "" {
		t.Fatalf("开放题不应虚构答案: %#v", items)
	}
}

func TestParseLecturePromptsQuestionPrefix(t *testing.T) {
	body := "## 题 1（理解）　词法单元为什么必须是\"一对东西\"？\n\n### 提示\n\n先想清楚。\n\n### 解答\n\n因为要同时记录类型与值。\n\n## 题 2（动手）　加上 `%` 约束\n\n### 解答\n\n改扫描器。\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if items[0].answer != "因为要同时记录类型与值。" {
		t.Fatalf("第1题答案=%q", items[0].answer)
	}
	if items[1].answer != "改扫描器。" {
		t.Fatalf("第2题答案=%q", items[1].answer)
	}
	for _, item := range items {
		if strings.Contains(item.prompt, "先想清楚") {
			t.Fatalf("提示不应进入题干: %q", item.prompt)
		}
	}
}

func TestParseLecturePromptsCollectedAnswers(t *testing.T) {
	body := "## 题目1：指针是什么？\n\n请说明。\n\n## 题目2：值传递与指针传递？\n\n请比较。\n\n## 参考答案\n\n### 题目1：指针是什么？\n\n**参考答案**：\n\n指针保存地址。\n\n### 题目2：值传递与指针传递？\n\n**参考答案**：\n\n一个复制值，一个复制地址。\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if items[0].answer != "指针保存地址。" {
		t.Fatalf("第1题答案=%q", items[0].answer)
	}
	if items[1].answer != "一个复制值，一个复制地址。" {
		t.Fatalf("第2题答案=%q", items[1].answer)
	}
}

func TestParseLecturePromptsVariants(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantQ, wantA int
	}{
		{"chinese", "## 题一：甲\n\n题一题干\n\n**解答**\n甲答\n\n## 题二：乙\n\n题二题干\n\n**解答**\n乙答\n", 2, 2},
		{"thought-prefix", "## 思考题 ①：甲\n\n题干\n\n**【解答】**\n答案\n", 1, 1},
		{"h3-question", "## 一、必做\n\n### 第 1 题　甲\n\n题干\n\n**参考要点**\n要点\n", 1, 1},
		{"judge", "## 第 1 题｜甲\n\n题干\n\n**怎么算答对了**：判据\n", 1, 1},
		{"summary", "## 题目1：甲\n\n<details>\n<summary>参考答案</summary>\n\n答案\n</details>\n", 1, 1},
		{"measured", "## 第 1 题　甲\n\n题干\n\n**解答（实测）**\n答案\n", 1, 1},
		{"hint", "## 题 1　甲\n\n### 提示\n\n提示内容\n\n### 解答\n\n答案\n", 1, 1},
	}
	for _, tc := range cases {
		items := parseLecturePrompts(tc.body)
		if len(items) != tc.wantQ {
			t.Errorf("%s: 题数=%d 期望 %d", tc.name, len(items), tc.wantQ)
			continue
		}
		answered := 0
		for _, item := range items {
			if item.answer != "" {
				answered++
			}
		}
		if answered != tc.wantA {
			t.Errorf("%s: 有答案题数=%d 期望 %d (%#v)", tc.name, answered, tc.wantA, items)
		}
	}
}

func TestLessonNumberHelpers(t *testing.T) {
	for input, want := range map[string]int{"一": 1, "九": 9, "十": 10, "十一": 11, "二十": 20, "二十三": 23} {
		if got := chineseNumber(input); got != want {
			t.Errorf("chineseNumber(%q)=%d，期望 %d", input, got, want)
		}
	}
	if circledNumber("⑤") != 5 || circledNumber("x") != 0 {
		t.Fatal("圈号解析错误")
	}
	for line, want := range map[string]int{
		"## 第 12 题": 12,
		"## ③":      3,
		"## 思考题 ④":  4,
		"## 题七":     7,
		"## 题目3：":   3,
		"## 基础题":    0,
	} {
		if got := numberInQuestion(line); got != want {
			t.Errorf("numberInQuestion(%q)=%d，期望 %d", line, got, want)
		}
	}
	if number := itemNumber([]string{"", "2", "", "", ""}); number != 2 {
		t.Fatalf("itemNumber=%d", number)
	}
}

func TestAnswerAndHintMarkers(t *testing.T) {
	for line, want := range map[string]string{
		"**参考答案**：回答":             "回答",
		"**完整参考答案**：完整":           "完整",
		"**解答（实测）**":              "",
		"【解答】":                    "",
		"### 解答":                  "",
		"<summary>参考答案</summary>": "",
	} {
		rest, ok := answerMarker(line)
		if !ok || rest != want {
			t.Errorf("answerMarker(%q)=(%q,%v)，期望 (%q,true)", line, rest, ok, want)
		}
	}
	if _, ok := answerMarker("普通题干"); ok {
		t.Fatal("普通题干不应识别为答案标记")
	}
	for _, line := range []string{"### 提示", "**【提示】**", "【提示】", "<summary>提示</summary>"} {
		if !hintMarker(line) {
			t.Errorf("hintMarker(%q) 应为 true", line)
		}
	}
	if hintMarker("普通题干") {
		t.Fatal("普通题干不应识别为提示")
	}
}

func TestCleanQuestionAndAnswer(t *testing.T) {
	if got := cleanQuestionHeading("## 🤔 第1题：甲"); got != "第1题：甲" {
		t.Fatalf("cleanQuestionHeading=%q", got)
	}
	if got := cleanAnswerText("**参考答案**：\n\n答案\n</details>\n---"); got != "答案" {
		t.Fatalf("cleanAnswerText=%q", got)
	}
}

func TestParseLecturePromptsSummaryAndJudge(t *testing.T) {
	body := "## 题目1：代码副本\n\n<details>\n<summary>参考答案</summary>\n\n静态库会把代码复制一份。\n</details>\n\n## 第 2 题｜确认号\n\n**怎么算答对了**：三个 seq 是 1 / 2 / 3。\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if !strings.Contains(items[0].answer, "复制一份") {
		t.Fatalf("summary 答案未提取: %q", items[0].answer)
	}
	if !strings.Contains(items[1].answer, "三个 seq") {
		t.Fatalf("判据答案未提取: %q", items[1].answer)
	}
}

func TestParseLecturePromptsReferenceSection(t *testing.T) {
	body := "# 第1讲：表达式 —— 思考题\n\n## 🤔 第1题：编译与解释\n\n请思考编译与解释的区别。\n\n## 🤔 第2题：链接的代价\n\n请推演静态链接。\n\n## 💡 参考思路（别急着看，先自己想！）\n\n<details>\n<summary>👉 点击展开参考思路</summary>\n\n### 第1题：编译与解释\n\n编译快，解释灵活。\n\n### 第2题：链接的代价\n\n静态链接体积大。\n\n</details>\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 {
		t.Fatalf("题数=%d", len(items))
	}
	if items[0].answer != "编译快，解释灵活。" {
		t.Fatalf("第1题答案=%q", items[0].answer)
	}
	if items[1].answer != "静态链接体积大。" {
		t.Fatalf("第2题答案=%q", items[1].answer)
	}
}
