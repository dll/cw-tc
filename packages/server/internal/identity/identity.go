package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"regexp"
)

var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

func ValidatePhone(phone string) error {
	if !phonePattern.MatchString(phone) {
		return errors.New("手机号格式非法")
	}
	return nil
}

func ValidateIDCard(id string) error {
	if len(id) != 18 {
		return errors.New("身份证号长度非法")
	}
	for i, r := range id {
		if i < 17 && (r < '0' || r > '9') {
			return errors.New("身份证号格式非法")
		}
	}
	last := id[17]
	if !((last >= '0' && last <= '9') || last == 'X' || last == 'x') {
		return errors.New("身份证号校验位格式非法")
	}
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checks := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(id[i]-'0') * weights[i]
	}
	want := checks[sum%11]
	got := last
	if got == 'x' {
		got = 'X'
	}
	if got != want {
		return errors.New("身份证号校验位错误")
	}
	return nil
}

func ProtectIDCard(id string, encKey, hmacKey []byte) (ciphertext, digest []byte, err error) {
	if err = ValidateIDCard(id); err != nil {
		return nil, nil, err
	}
	if len(encKey) != 32 {
		return nil, nil, errors.New("身份证加密密钥必须为 32 字节")
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	sealed := gcm.Seal(nil, nonce, []byte(id), nil)
	ciphertext = append(nonce, sealed...)
	mac := hmac.New(sha256.New, hmacKey)
	_, _ = mac.Write([]byte(id))
	return ciphertext, mac.Sum(nil), nil
}

func UnprotectIDCard(ciphertext, encKey []byte) (string, error) {
	if len(encKey) != 32 {
		return "", errors.New("身份证加密密钥必须为 32 字节")
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("身份证密文长度非法")
	}
	plain, err := gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("身份证密文解密失败: %w", err)
	}
	return string(plain), nil
}
