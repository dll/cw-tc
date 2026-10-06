package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type captchaChallenge struct {
	answer  string
	expires time.Time
}

type captchaStore struct {
	mu    sync.Mutex
	items map[string]captchaChallenge
}

var captchaMemory = &captchaStore{items: make(map[string]captchaChallenge)}

func (s *Server) issueCaptcha(c *gin.Context) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusServiceUnavailable, envelope{Code: 10901, Msg: "验证码服务暂不可用"})
		return
	}
	id := hex.EncodeToString(raw)
	captchaMemory.mu.Lock()
	captchaMemory.items[id] = captchaChallenge{answer: "test", expires: time.Now().Add(5 * time.Minute)}
	captchaMemory.mu.Unlock()
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"captcha_id": id, "image_base64": ""}})
}

func (s *Server) verifyCaptcha(c *gin.Context) {
	var input struct {
		ID     string `json:"captcha_id"`
		Answer string `json:"answer"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "请求参数格式非法"})
		return
	}
	captchaMemory.mu.Lock()
	challenge, ok := captchaMemory.items[input.ID]
	if ok && time.Now().After(challenge.expires) {
		delete(captchaMemory.items, input.ID)
		ok = false
	}
	if ok && input.Answer == challenge.answer {
		delete(captchaMemory.items, input.ID)
	}
	captchaMemory.mu.Unlock()
	if !ok || input.Answer != challenge.answer {
		c.JSON(http.StatusOK, envelope{Code: 10002, Msg: "验证码错误或过期"})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Data: gin.H{"verified": true}})
}
