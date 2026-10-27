package api

import "testing"

func TestDiscussionBodyPrivacy(t *testing.T) {
	for _, body := range []string{"讨论这讲的网络结构", "联系我 13812345678", "样例 110101199001011234", "测评答案如下"} {
		ok := discussionBodyOK(body)
		if ok != (body == "讨论这讲的网络结构") { t.Fatalf("内容校验异常: %q: %v", body, ok) }
	}
}
