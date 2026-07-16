package config

import "testing"

func TestParseQQConfigDefaultsFramework(t *testing.T) {
	config, err := ParseQQConfig(`{"server_url":"ws://127.0.0.1:3001","access_token":" ws-token ","http_api_access_token":" http-token "}`)
	if err != nil {
		t.Fatal(err)
	}
	if config.Framework != "napcat" || config.ServerURL != "ws://127.0.0.1:3001" || config.AccessToken != "ws-token" || config.HTTPAPIAccessToken != "http-token" {
		t.Fatalf("config = %+v", config)
	}
}

func TestParseQQConfigAcceptsSupportedSchemes(t *testing.T) {
	tests := []struct {
		name       string
		serverURL  string
		httpAPIURL string
	}{
		{name: "ws and http", serverURL: "ws://127.0.0.1:3001", httpAPIURL: "http://127.0.0.1:3000"},
		{name: "wss and https", serverURL: "wss://onebot.example/ws", httpAPIURL: "https://onebot.example/api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseQQConfig(`{"framework":"napcat","server_url":"` + tt.serverURL + `","http_api_url":"` + tt.httpAPIURL + `"}`)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParseQQConfigMigratesLegacyAPIURLByScheme(t *testing.T) {
	wsConfig, err := ParseQQConfig(`{"api_url":"wss://onebot.example/ws"}`)
	if err != nil {
		t.Fatal(err)
	}
	if wsConfig.ServerURL != "wss://onebot.example/ws" || wsConfig.HTTPAPIURL != "" {
		t.Fatalf("ws config = %+v", wsConfig)
	}

	httpConfig, err := ParseQQConfig(`{"server_url":"ws://onebot.example/ws","api_url":"https://onebot.example/api"}`)
	if err != nil {
		t.Fatal(err)
	}
	if httpConfig.ServerURL != "ws://onebot.example/ws" || httpConfig.HTTPAPIURL != "https://onebot.example/api" {
		t.Fatalf("http config = %+v", httpConfig)
	}
}

func TestParseQQConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unknown framework", raw: `{"framework":"llonebot","server_url":"ws://127.0.0.1:3001"}`},
		{name: "missing server url", raw: `{"framework":"napcat"}`},
		{name: "server http", raw: `{"server_url":"http://127.0.0.1:3001"}`},
		{name: "server invalid", raw: `{"server_url":"ws://"}`},
		{name: "http api ws", raw: `{"server_url":"ws://127.0.0.1:3001","http_api_url":"ws://127.0.0.1:3000"}`},
		{name: "http api query", raw: `{"server_url":"ws://127.0.0.1:3001","http_api_url":"http://127.0.0.1:3000/api?token=x"}`},
		{name: "server fragment", raw: `{"server_url":"ws://127.0.0.1:3001/ws#event"}`},
		{name: "legacy unsupported", raw: `{"api_url":"ftp://127.0.0.1/resource"}`},
		{name: "legacy http without websocket", raw: `{"api_url":"http://127.0.0.1:3000"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseQQConfig(tt.raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
