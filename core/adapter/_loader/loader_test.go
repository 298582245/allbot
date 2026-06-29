package loader

import (
	"testing"

	"github.com/allbot/allbot/core/adapter/_registry"
)

func TestLoaderRegistersAdapterManifests(t *testing.T) {
	for _, platform := range []string{"dingtalk", "qq", "qq_office", "telegram", "wechat_official"} {
		if _, ok := registry.Get(platform); !ok {
			t.Fatalf("loader did not register platform %s", platform)
		}
	}
}
