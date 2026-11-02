package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrSMSCooldown = errors.New("短信发送过于频繁")
var ErrSMSInvalid = errors.New("短信验证码错误或过期")

type SMSService struct {
	mu        sync.Mutex
	codes     map[string]smsRecord
	daily     map[string]int
	dailyDate string
}

type smsRecord struct {
	digest    string
	createdAt time.Time
}

func NewSMSService() *SMSService {
	return &SMSService{codes: make(map[string]smsRecord), daily: make(map[string]int)}
}

func (s *SMSService) Issue(_ context.Context, phone, scene, code string, now time.Time) (string, error) {
	if ValidatePhone(phone) != nil || scene == "" || len(code) != 6 {
		return "", ErrInvalidInput
	}
	key := phone + ":" + scene
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.codes[key]; ok && now.Sub(previous.createdAt) < 60*time.Second {
		return "", ErrSMSCooldown
	}
	digest := sha256.Sum256([]byte(code))
	s.codes[key] = smsRecord{digest: hex.EncodeToString(digest[:]), createdAt: now}
	return "", nil
}

func (s *SMSService) Consume(phone, scene, code string, now time.Time) error {
	key := phone + ":" + scene
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.codes[key]
	if !ok || now.Sub(record.createdAt) > 10*time.Minute {
		return ErrSMSInvalid
	}
	digest := sha256.Sum256([]byte(code))
	if hex.EncodeToString(digest[:]) != record.digest {
		return ErrSMSInvalid
	}
	delete(s.codes, key)
	return nil
}

func (s *SMSService) Verify(phone, scene, code string, now time.Time) bool {
	key := phone + ":" + scene
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.codes[key]
	if !ok || now.Sub(record.createdAt) > 10*time.Minute {
		return false
	}
	digest := sha256.Sum256([]byte(code))
	return hex.EncodeToString(digest[:]) == record.digest
}

// ClaimDaily 限制单手机号每日发送次数（默认 10 次），按服务端日期切分。
func (s *SMSService) ClaimDaily(phone string, now time.Time, limit int) bool {
	if s == nil || limit <= 0 {
		return false
	}
	date := now.UTC().Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.daily == nil || s.dailyDate != date {
		s.daily = make(map[string]int)
		s.dailyDate = date
	}
	if s.daily[phone] >= limit {
		return false
	}
	s.daily[phone]++
	return true
}

// Revoke 在短信渠道发送失败时撤销刚生成的验证码，避免“发送失败但码仍有效”。
func (s *SMSService) Revoke(phone, scene string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.codes, phone+":"+scene)
}
