package web

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

type openAPIIPRules struct {
	allowAll bool
	prefixes []netip.Prefix
	raw      []string
}

type openAPIAccessConfig struct {
	global         openAPIIPRules
	globalSource   string
	trustedProxies openAPIIPRules
	retentionDays  int
}

func defaultOpenAPIAccessConfig() *openAPIAccessConfig {
	settings := config.DefaultOpenAPISettings()
	global, _ := compileOpenAPIIPRules(settings.IPWhitelist, true, false)
	trusted, _ := compileOpenAPIIPRules(settings.TrustedProxies, false, true)
	return &openAPIAccessConfig{global: global, globalSource: "default", trustedProxies: trusted, retentionDays: settings.RetentionDays}
}

func compileOpenAPIIPRules(values []string, allowWildcard, rejectEmpty bool) (openAPIIPRules, error) {
	rules := openAPIIPRules{raw: make([]string, 0, len(values)), prefixes: make([]netip.Prefix, 0, len(values))}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return openAPIIPRules{}, fmt.Errorf("IP 规则不能为空")
		}
		if value == "*" {
			if !allowWildcard {
				return openAPIIPRules{}, fmt.Errorf("可信代理不允许使用 *")
			}
			if len(values) != 1 {
				return openAPIIPRules{}, fmt.Errorf("* 只能作为唯一规则")
			}
			rules.allowAll = true
			rules.raw = []string{"*"}
			return rules, nil
		}
		prefix, err := parseOpenAPIIPPrefix(value)
		if err != nil {
			return openAPIIPRules{}, fmt.Errorf("IP 规则 %q 无效: %w", value, err)
		}
		normalized := prefix.String()
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		rules.raw = append(rules.raw, normalized)
		rules.prefixes = append(rules.prefixes, prefix)
	}
	if rejectEmpty && len(rules.prefixes) == 0 {
		return openAPIIPRules{}, fmt.Errorf("IP 规则至少需要一项")
	}
	return rules, nil
}

func parseOpenAPIIPPrefix(value string) (netip.Prefix, error) {
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, err
		}
		addr := prefix.Addr()
		bits := prefix.Bits()
		if addr.Is4In6() {
			addr = addr.Unmap()
			bits -= 96
			if bits < 0 || bits > 32 {
				return netip.Prefix{}, fmt.Errorf("IPv4 映射前缀长度无效")
			}
		}
		return netip.PrefixFrom(addr, bits).Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	if addr.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("不允许 IPv6 zone")
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func (rules openAPIIPRules) contains(addr netip.Addr) bool {
	if rules.allowAll {
		return true
	}
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range rules.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (s *Server) initializeOpenAPIAccess() {
	s.openAPIAccess.Store(defaultOpenAPIAccessConfig())
	if err := s.refreshOpenAPIAccess(); err != nil {
		return
	}
}

func (s *Server) refreshOpenAPIAccess() error {
	database := s.runtimeDatabase()
	if database == nil {
		if s.openAPIAccess.Load() == nil {
			s.openAPIAccess.Store(defaultOpenAPIAccessConfig())
		}
		return nil
	}
	settings, err := database.GetOpenAPISettings()
	if err != nil {
		return err
	}
	compiled, err := compileOpenAPIAccessConfig(settings)
	if err != nil {
		return err
	}
	if _, err := database.GetSetting("open_api.config"); err != nil {
		compiled.globalSource = "default"
	}
	s.openAPIAccess.Store(compiled)
	return nil
}

func compileOpenAPIAccessConfig(settings config.OpenAPISettings) (*openAPIAccessConfig, error) {
	global, err := compileOpenAPIIPRules(settings.IPWhitelist, true, true)
	if err != nil {
		return nil, fmt.Errorf("全局 IP 白名单无效: %w", err)
	}
	trusted, err := compileOpenAPIIPRules(settings.TrustedProxies, false, false)
	if err != nil {
		return nil, fmt.Errorf("可信代理无效: %w", err)
	}
	if settings.RetentionDays < 0 || settings.RetentionDays > 3650 {
		return nil, fmt.Errorf("调用日志保留天数必须在 0 到 3650 之间")
	}
	return &openAPIAccessConfig{global: global, globalSource: "global", trustedProxies: trusted, retentionDays: settings.RetentionDays}, nil
}

func (s *Server) currentOpenAPIAccess() *openAPIAccessConfig {
	if value := s.openAPIAccess.Load(); value != nil {
		return value
	}
	value := defaultOpenAPIAccessConfig()
	s.openAPIAccess.Store(value)
	return value
}

func (s *Server) resolveOpenAPIClientIP(r *http.Request) netip.Addr {
	access := s.currentOpenAPIAccess()
	remote := parseOpenAPIAddress(r.RemoteAddr)
	if !remote.IsValid() || !access.trustedProxies.contains(remote) {
		return remote
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := parseOpenAPIAddress(forwarded[index])
		if !candidate.IsValid() {
			continue
		}
		if access.trustedProxies.contains(candidate) {
			continue
		}
		return candidate
	}
	if candidate := parseOpenAPIAddress(r.Header.Get("X-Real-IP")); candidate.IsValid() {
		return candidate
	}
	return remote
}

func parseOpenAPIAddress(value string) netip.Addr {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Addr{}
	}
	if address, err := netip.ParseAddrPort(value); err == nil {
		return address.Addr().Unmap()
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}
	}
	return address.Unmap()
}

func (s *Server) effectiveOpenAPIIPRules(endpoint *types.OpenAPIEndpoint) (openAPIIPRules, string, string, error) {
	if endpoint != nil && endpoint.IPWhitelist != nil {
		rules, err := compileOpenAPIIPRules(*endpoint.IPWhitelist, true, true)
		if err != nil {
			return openAPIIPRules{}, "", "endpoint", err
		}
		mode := "custom"
		if rules.allowAll {
			mode = "allow_all"
		}
		return rules, mode, "endpoint", nil
	}
	access := s.currentOpenAPIAccess()
	return access.global, "inherit", access.globalSource, nil
}

func (s *Server) openAPIIPAllowed(endpoint *types.OpenAPIEndpoint, clientIP netip.Addr) bool {
	rules, _, _, err := s.effectiveOpenAPIIPRules(endpoint)
	return err == nil && rules.contains(clientIP)
}

// atomicOpenAPIAccess 封装类型参数，避免 Server 字段暴露具体同步细节。
type atomicOpenAPIAccess struct {
	value atomic.Pointer[openAPIAccessConfig]
}

func (a *atomicOpenAPIAccess) Load() *openAPIAccessConfig       { return a.value.Load() }
func (a *atomicOpenAPIAccess) Store(value *openAPIAccessConfig) { a.value.Store(value) }
