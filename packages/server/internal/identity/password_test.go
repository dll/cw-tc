package identity

import "testing"

func TestValidatePasswordRequiresLengthAndMix(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"short1", true},   // 6 位过短
		{"abcdefgh", true}, // 无数字
		{"12345678", true}, // 无字母
		{"abc12345", false},
		{"verylongpassword0000000", true}, // 超过 20 位
	}
	for _, c := range cases {
		if err := ValidatePassword(c.password); (err != nil) != c.wantErr {
			t.Fatalf("ValidatePassword(%q) = %v, wantErr=%v", c.password, err, c.wantErr)
		}
	}
}

func TestValidatePasswordAgainstIDCardBlocksWeakValues(t *testing.T) {
	idCard := "110101199001011237" // 后 6 位 011237，生日 19900101
	if err := ValidatePasswordAgainstIDCard("abc011237", idCard); err == nil {
		t.Fatal("密码含身份证后 6 位应被拦截")
	}
	if err := ValidatePasswordAgainstIDCard("x19900101y", idCard); err == nil {
		t.Fatal("密码含生日应被拦截")
	}
	if err := ValidatePasswordAgainstIDCard("abc12345", idCard); err != nil {
		t.Fatalf("合法密码不应被拦截: %v", err)
	}
}
