package identity

import "errors"

// ValidatePassword 校验密码强度：8~20 位且同时含字母与数字。
// 弱口令拦截（身份证后 6 位 / 生日）在调用方拿到身份证号后由 ValidatePasswordAgainstIDCard 追加判断。
func ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 20 {
		return errors.New("密码长度需为 8~20 位")
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("密码需同时包含字母与数字")
	}
	return nil
}

// ValidatePasswordAgainstIDCard 在基本强度之上追加“不得使用身份证后 6 位或生日”拦截。
func ValidatePasswordAgainstIDCard(password, idCard string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	if len(idCard) != 18 {
		return nil
	}
	id6 := idCard[12:18]
	if len(password) >= 6 && contains(password, id6) {
		return errors.New("密码不得包含身份证后 6 位")
	}
	birth := idCard[6:14]
	if contains(password, birth) {
		return errors.New("密码不得包含生日")
	}
	return nil
}
