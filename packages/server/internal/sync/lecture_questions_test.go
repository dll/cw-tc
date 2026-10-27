package sync

import "testing"

func TestParseLecturePromptsWithSolutions(t *testing.T) {
	body := "# 思考题\n## ① 为什么分层？\n提示文字\n**解答**\n因为隔离变化。\n---\n## ② 下一题？\n**答案**\n第二题解答。\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 { t.Fatalf("题数=%d", len(items)) }
	if items[0].prompt == "" || items[0].answer == "" || items[1].answer == "" { t.Fatalf("提取结果不完整: %#v", items) }
}

func TestParseLecturePromptsWithoutSolutions(t *testing.T) {
	body := "## 基础题\n**1. 哪些操作会引发系统调用？**\n- 读取文件\n---\n**2. 为什么？**\n请说明理由\n"
	items := parseLecturePrompts(body)
	if len(items) != 2 { t.Fatalf("题数=%d", len(items)) }
	if items[0].answer != "" || items[1].answer != "" { t.Fatalf("开放题不应虚构答案: %#v", items) }
}
