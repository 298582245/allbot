package config

import "testing"

func TestParseQQOfficeConfigDefaultsAndValidation(t *testing.T) {
	config, err := ParseQQOfficeConfig(`{"app_id":" app123 ","client_secret":" secret456 "}`)
	if err != nil {
		t.Fatalf("ParseQQOfficeConfig returned error: %v", err)
	}
	if config.AppID != "app123" || config.ClientSecret != "secret456" {
		t.Fatalf("config = %+v", config)
	}
	if config.APIBaseURL != "https://api.sgroup.qq.com" {
		t.Fatalf("APIBaseURL = %q", config.APIBaseURL)
	}
	if config.TokenURL != "https://bots.qq.com/app/getAppAccessToken" {
		t.Fatalf("TokenURL = %q", config.TokenURL)
	}
}

func TestParseQQOfficeConfigRequiresCredentials(t *testing.T) {
	if _, err := ParseQQOfficeConfig(`{"client_secret":"secret456"}`); err == nil {
		t.Fatal("expected app_id validation error")
	}
	if _, err := ParseQQOfficeConfig(`{"app_id":"app123"}`); err == nil {
		t.Fatal("expected client_secret validation error")
	}
}
