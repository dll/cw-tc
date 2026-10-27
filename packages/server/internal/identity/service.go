package identity

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidInput          = errors.New("注册参数非法")
	ErrDatabaseNotConfigured = errors.New("账号数据库未配置")
	ErrPhoneRegistered       = errors.New("手机号已注册")
	ErrIDCardRegistered      = errors.New("身份证号已注册")
)

type RegisterInput struct {
	Name     string
	Phone    string
	IDCard   string
	Password string
	// SMSCode/SMSScene 保留用于兼容旧调用方；注册链路不再消费短信验证码。
	SMSCode       string
	SMSScene      string
	RequireReview bool // HTTP 注册链路要求运营审核；内部导入可直接建立已审核账号。
}

type Student struct {
	ID        int64
	StudentNo string
	Name      string
	Phone     string
}

type Service struct {
	db      *sql.DB
	encKey  []byte
	hmacKey []byte
	sms     *SMSService
}

func NewService(database *sql.DB, encKey, hmacKey []byte) *Service {
	return &Service{db: database, encKey: encKey, hmacKey: hmacKey}
}

// WithSMS 注入短信服务，启用验证码校验；未注入时相关能力 fail-closed。
func (s *Service) WithSMS(sms *SMSService) *Service { s.sms = sms; return s }

func (s *Service) Register(ctx context.Context, input RegisterInput) (Student, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.IDCard = strings.ToUpper(input.IDCard)
	if input.Name == "" || ValidatePhone(input.Phone) != nil || (input.IDCard != "" && ValidateIDCard(input.IDCard) != nil) || input.Password == "" {
		return Student{}, ErrInvalidInput
	}
	if err := ValidatePasswordAgainstIDCard(input.Password, input.IDCard); err != nil {
		return Student{}, ErrInvalidInput
	}
	var phoneVerifiedAt any
	// 手机号审核由运营管理员完成；注册阶段不调用短信服务，也不写入已验证时间。
	if s.db == nil {
		return Student{}, ErrDatabaseNotConfigured
	}
	var ciphertext, digest []byte
	var err error
	if input.IDCard != "" {
		ciphertext, digest, err = ProtectIDCard(input.IDCard, s.encKey, s.hmacKey)
		if err != nil {
			return Student{}, fmt.Errorf("保护身份证号失败: %w", err)
		}
	}
	studentNo := fmt.Sprintf("S%08d", checksumStudentNo(input.Phone, input.IDCard))
	// 注册必须记录"同意的协议版本"：数据字典 §2.1 的 `agreement_versions` 是 **NOT NULL**，
	// 且合规上要有据可查（铁律 3）。空值或空数组都不行 —— 那等于"同意过什么"查不到。
	// TODO(M1-04)：版本号应取自 `/public/site` 当前生效版本，**不要长期写死**（文档改版会漂移）。
	agreements := `[{"doc":"user_agreement","version":"v1.0"},{"doc":"privacy_policy","version":"v1.0"}]`
	var id int64
	reviewStatus := "approved"
	if input.RequireReview {
		reviewStatus = "pending"
	}
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO student(student_no, name, phone, phone_verified_at, registration_review_status, id_card_hash, id_card_cipher, password_hash, agreement_versions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		studentNo, input.Name, input.Phone, phoneVerifiedAt, reviewStatus, digest, ciphertext, hashPassword(input.Password), agreements).Scan(&id)
	if err != nil {
		// 唯一约束报错跨方言：SQLite 是 `UNIQUE constraint failed: student.phone`，
		// PostgreSQL 是 `duplicate key ... "uq_student_phone"`，两套都要匹配。
		if stringsContains(err.Error(), "student.phone") || stringsContains(err.Error(), "uq_student_phone") {
			return Student{}, ErrPhoneRegistered
		}
		if stringsContains(err.Error(), "id_card_hash") || stringsContains(err.Error(), "uq_student_id_card_hash") {
			return Student{}, ErrIDCardRegistered
		}
		return Student{}, fmt.Errorf("注册写入失败: %w", err)
	}
	return Student{ID: id, StudentNo: studentNo, Name: input.Name, Phone: input.Phone}, nil
}

func hashPassword(password string) string {
	digest := sha256.Sum256([]byte(password))
	return hex.EncodeToString(digest[:])
}

func checksumStudentNo(phone, id string) int64 {
	var sum int64
	for _, value := range phone + id {
		sum = (sum*31 + int64(value)) % 100000000
	}
	return sum
}

func stringsContains(value, target string) bool {
	return len(target) > 0 && len(value) >= len(target) && contains(value, target)
}

func contains(value, target string) bool {
	for i := 0; i+len(target) <= len(value); i++ {
		if value[i:i+len(target)] == target {
			return true
		}
	}
	return false
}
