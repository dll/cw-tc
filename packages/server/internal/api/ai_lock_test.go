package api

import "testing"

func TestAILockAllowsSingleConcurrentRequest(t *testing.T) {
	server := &Server{}
	release, ok := server.acquireAILock(1)
	if !ok {
		t.Fatal("首次应获得锁")
	}
	if _, ok := server.acquireAILock(1); ok {
		t.Fatal("同一学员第二路应被拒")
	}
	release()
	release2, ok := server.acquireAILock(1)
	if !ok {
		t.Fatal("释放后应可再次获得锁")
	}
	release2()
	if _, ok := server.acquireAILock(0); ok {
		t.Fatal("非法学员号不应获得锁")
	}
}
