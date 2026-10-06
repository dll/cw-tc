package identity

import "testing"

func TestValidatePhoneAndIDCard(t *testing.T) {
	if err := ValidatePhone("13812345678"); err != nil {
		t.Fatalf("合法手机号被拒绝: %v", err)
	}
	if err := ValidatePhone("12812345678"); err == nil {
		t.Fatal("非法号段应被拒绝")
	}
	if err := ValidateIDCard("110101199001011237"); err != nil {
		t.Fatalf("合法样例身份证被拒绝: %v", err)
	}
	if err := ValidateIDCard("110101199001011235"); err == nil {
		t.Fatal("校验位错误的身份证应被拒绝")
	}
}

func TestProtectIDCardRoundTripAndStableHash(t *testing.T) {
	cipher, hash, err := ProtectIDCard("110101199001011237", []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if string(cipher) == "110101199001011237" || len(hash) == 0 {
		t.Fatal("不得保存身份证明文且必须生成哈希")
	}
	plain, err := UnprotectIDCard(cipher, []byte("01234567890123456789012345678901"))
	if err != nil || plain != "110101199001011237" {
		t.Fatalf("解密结果 = %q, err=%v", plain, err)
	}
	_, hash2, err := ProtectIDCard("110101199001011237", []byte("01234567890123456789012345678901"), []byte("hmac-key-for-cwtc-tests-012345678901234"))
	if err != nil || string(hash) != string(hash2) {
		t.Fatal("同一身份证号的 HMAC 去重值必须稳定")
	}
}
