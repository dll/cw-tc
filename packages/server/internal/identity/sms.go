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
	mu    sync.Mutex
	codes map[string]smsRecord
}

type smsRecord struct {
	digest    string
	createdAt time.Time
}

func NewSMSService() *SMSService {
	return &SMSService{codes: make(map[string]smsRecord)}
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
