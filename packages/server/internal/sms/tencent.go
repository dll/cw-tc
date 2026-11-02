// Package sms 对接腾讯云短信（TC3-HMAC-SHA256），不引入厂商 SDK。
package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("短信渠道未配置")

type Tencent struct {
	SecretID   string
	SecretKey  string
	SdkAppID   string
	SignName   string
	TemplateID string
	Region     string
	Endpoint   string
	Client     *http.Client
}

func (t *Tencent) Send(ctx context.Context, phone, code string) error {
	if t == nil || t.SecretID == "" || t.SecretKey == "" || t.SdkAppID == "" || t.SignName == "" || t.TemplateID == "" {
		return ErrNotConfigured
	}
	phone = strings.TrimSpace(phone)
	if len(phone) != 11 || !strings.HasPrefix(phone, "1") {
		return fmt.Errorf("手机号格式非法")
	}
	region := t.Region
	if region == "" {
		region = "ap-guangzhou"
	}
	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = "https://sms.tencentcloudapi.com"
	}
	payload, _ := json.Marshal(map[string]any{
		"PhoneNumberSet":   []string{"+86" + phone},
		"SmsSdkAppId":      t.SdkAppID,
		"SignName":         t.SignName,
		"TemplateId":       t.TemplateID,
		"TemplateParamSet": []string{code},
	})
	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	timestamp := now.Unix()
	contentType := "application/json; charset=utf-8"
	host := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	hashedPayload := sha256Hex(payload)
	canonical := "POST\n/\n\ncontent-type:" + contentType + "\nhost:" + host + "\n\ncontent-type;host\n" + hashedPayload
	stringToSign := "TC3-HMAC-SHA256\n" + fmt.Sprint(timestamp) + "\n" + date + "/sms/tc3_request\n" + sha256Hex([]byte(canonical))
	secretDate := hmacSHA256([]byte("TC3"+t.SecretKey), date)
	secretService := hmacSHA256(secretDate, "sms")
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	authorization := "TC3-HMAC-SHA256 Credential=" + t.SecretID + "/" + date + "/sms/tc3_request, SignedHeaders=content-type;host, Signature=" + signature

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Host", host)
	request.Header.Set("X-TC-Action", "SendSms")
	request.Header.Set("X-TC-Version", "2021-01-11")
	request.Header.Set("X-TC-Timestamp", fmt.Sprint(timestamp))
	request.Header.Set("X-TC-Region", region)
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("短信渠道 HTTP %d", response.StatusCode)
	}
	var result struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
			SendStatusSet []struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("短信渠道响应非法")
	}
	if result.Response.Error != nil {
		return fmt.Errorf("短信发送失败：%s %s", result.Response.Error.Code, result.Response.Error.Message)
	}
	if len(result.Response.SendStatusSet) == 0 || result.Response.SendStatusSet[0].Code != "Ok" {
		return fmt.Errorf("短信发送未成功")
	}
	return nil
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
