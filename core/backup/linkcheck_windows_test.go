//go:build windows

package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidatePathWithoutLinksRejectsJunction(t *testing.T) {
	workspace := t.TempDir()
	target := filepath.Join(workspace, "target")
	junction := filepath.Join(workspace, "junction")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput()
	if err != nil {
		t.Skipf("当前环境不支持 junction 测试: %v: %s", err, output)
	}
	if err := validatePathWithoutLinks(filepath.Join(junction, "child"), true); err == nil {
		t.Fatal("包含 junction 的路径应被拒绝")
	}
}
