package internet

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"yemaka/internal/safety"
)

func (s *Service) validatePolicy(input FetchInput, target *url.URL) error {
	if s.Config.DefaultMode == "off" && !input.TaskApproved {
		return fmt.Errorf("internet default mode is off")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are allowed")
	}
	request := safety.PolicyRequest{
		Domain:         safety.DomainInternet,
		Action:         safety.ActionFetch,
		Level:          safety.LevelConfirm,
		PolicyMode:     s.PolicyMode,
		Actor:          input.Caller,
		Resource:       target.Hostname(),
		Enabled:        s.Config.Enabled,
		ProfileEnabled: s.Config.Enabled && s.Config.DefaultMode == "profile_enabled",
		TaskApproved:   input.TaskApproved,
		Network:        true,
		Domains:        input.AllowedDomains,
	}
	decision := safety.EvaluatePolicy(request)
	s.auditPolicy(request, decision)
	if !decision.Allowed {
		return fmt.Errorf("%s", decision.Explanation)
	}
	if safety.IsFullAccessMode(s.PolicyMode) {
		return nil
	}
	if decision.RequiresConfirmation && !input.TaskApproved {
		return fmt.Errorf("%s", decision.Explanation)
	}
	if !methodAllowed(input.Method, s.Config.AllowMethods) && !(input.Method == http.MethodPost && input.AllowVendorAPI) {
		return fmt.Errorf("internet method %s is not allowed", input.Method)
	}
	switch input.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodPost:
		if !input.AllowVendorAPI {
			return fmt.Errorf("POST is reserved for connector-specific approval")
		}
	default:
		return fmt.Errorf("GET and HEAD are the only internet methods available in this milestone")
	}
	if len(input.AllowedDomains) > 0 && !domainAllowed(target.Hostname(), input.AllowedDomains) {
		return fmt.Errorf("domain %s is outside the task allowlist", target.Hostname())
	}
	if s.Config.Policy.BlockLocalNetworkByDefault && isLocalHostname(target.Hostname()) {
		return fmt.Errorf("local network hosts are blocked by default")
	}
	if s.Config.Policy.BlockPrivateIPRanges {
		if err := rejectPrivateAddress(target.Hostname()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) auditPolicy(request safety.PolicyRequest, decision safety.PolicyDecision) {
	if !s.Config.Policy.LogRequests || strings.TrimSpace(s.LogPath) == "" {
		return
	}
	_ = safety.AppendPolicyAudit(filepath.Dir(s.LogPath), request, decision)
}

func parseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is required")
	}
	target, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("url must include scheme and host")
	}
	return target, nil
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet
	}
	return method
}

func methodAllowed(method string, allowed []string) bool {
	if len(allowed) == 0 {
		allowed = []string{http.MethodGet, http.MethodHead}
	}
	for _, item := range allowed {
		if strings.ToUpper(strings.TrimSpace(item)) == method {
			return true
		}
	}
	return false
}

func domainAllowed(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, domain := range allowed {
		domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
		if domain == "" {
			continue
		}
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func isLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local")
}

func rejectPrivateAddress(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("private/local IP ranges are blocked: %s", host)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host for private IP check: %w", err)
	}
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("private/local IP ranges are blocked: %s", host)
		}
	}
	return nil
}

func isPrivateOrLocalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}
