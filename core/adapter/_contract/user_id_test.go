package contract

import "testing"

func TestNormalizeUserID(t *testing.T) {
	tests := map[string]string{
		"123456":                               "123456",
		"[CQ:at,qq=123456]":                    "123456",
		"[CQ:at,qq=123456,display=用户]":         "123456",
		"<@D95E233ED951C75CE1B3A13C58B62D19>":  "D95E233ED951C75CE1B3A13C58B62D19",
		"<@!D95E233ED951C75CE1B3A13C58B62D19>": "D95E233ED951C75CE1B3A13C58B62D19",
		`<a href="tg://user?id=123456">用户</a>`: "123456",
		"[User](tg://user?id=123456)":          "123456",
		"tg://user?id=123456":                  "123456",
		"@123456":                              "123456",
	}
	for input, want := range tests {
		if got := NormalizeUserID(input); got != want {
			t.Errorf("NormalizeUserID(%q) = %q, want %q", input, got, want)
		}
	}
}
