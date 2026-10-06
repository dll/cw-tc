package identity

import "testing"

func TestValidationRejectsMalformedAndWrongKey(t *testing.T) {
	for _, id := range []string{"", "11010119900101123", "110101199001011235", "11010119900101123Z"} {
		if err := ValidateIDCard(id); err == nil {
			t.Errorf("身份证 %q 应被拒绝", id)
		}
	}
	if _, err := UnprotectIDCard([]byte("short"), []byte("short")); err == nil {
		t.Fatal("短密钥或短密文必须被拒绝")
	}
}
