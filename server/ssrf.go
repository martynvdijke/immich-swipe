package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// outboundLookupTimeout bounds DNS resolution performed while validating a
// client-supplied server URL. It is a variable so tests can replace the
// resolver without touching the network.
var outboundLookupTimeout = 5 * time.Second

// lookupIPAddr is the DNS resolver used by SSRF validation. Tests replace it
// with a deterministic stub.
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// parseServerURL performs the cheap, non-network part of server URL
// validation: http/https only, a host is required, and credentials are not
// allowed in the URL.
func parseServerURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("server URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid server URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("server URL must use http or https")
	}
	if u.Hostname() == "" {
		return nil, errors.New("server URL must include a host")
	}
	if u.User != nil {
		return nil, errors.New("server URL must not include credentials")
	}
	return u, nil
}

// normalizeURLForCompare returns a canonical scheme://host[:port] string used
// to decide whether a client-supplied URL is the administrator-configured
// server. The configured URL is trusted so self-hosted Immich instances on
// private/LAN addresses keep working.
func normalizeURLForCompare(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func isConfiguredServerURL(candidate, configured string) bool {
	if configured == "" {
		return false
	}
	cu, err := parseServerURL(candidate)
	if err != nil {
		return false
	}
	gu, err := parseServerURL(configured)
	if err != nil {
		return false
	}
	return normalizeURLForCompare(cu) == normalizeURLForCompare(gu)
}

// isDisallowedIP reports whether an outbound connection to ip must be refused
// because it targets the local host, a private network, link-local/metadata
// services, or another non-public range.
func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// Normalize IPv4-mapped IPv6 (e.g. ::ffff:127.0.0.1) to plain IPv4.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// Carrier-grade NAT (100.64.0.0/10), "this network" (0.0.0.0/8),
		// benchmarking (198.18.0.0/15), and reserved 240.0.0.0/4.
		switch {
		case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127:
			return true
		case v4[0] == 0:
			return true
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19):
			return true
		case v4[0] >= 240:
			return true
		}
	}
	return false
}

// resolveAndCheck resolves host (or parses it when it is an IP literal) and
// rejects the URL when any answer points at a disallowed address.
func resolveAndCheck(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return errors.New("server URL resolves to a private or local address")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, outboundLookupTimeout)
	defer cancel()
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return errors.New("cannot resolve server URL host")
	}
	for _, addr := range addrs {
		if isDisallowedIP(addr.IP) {
			return errors.New("server URL resolves to a private or local address")
		}
	}
	return nil
}

// validateClientServerURL validates a URL supplied by a client. The
// administrator-configured IMMICH_SERVER_URL is trusted as-is (self-hosted
// deployments commonly live on private/LAN addresses); every other URL must
// resolve to a public address.
func (s *Server) validateClientServerURL(raw string) (*url.URL, error) {
	u, err := parseServerURL(raw)
	if err != nil {
		return nil, err
	}
	if isConfiguredServerURL(raw, s.config.ServerURL) {
		return u, nil
	}
	if err := resolveAndCheck(context.Background(), u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

// validatingRoundTripper re-validates every outgoing request URL (including
// redirect targets, which arrive as fresh requests) before handing it to the
// wrapped transport.
type validatingRoundTripper struct {
	base http.RoundTripper
	srv  *Server
}

func (v *validatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil {
		return nil, errors.New("missing request URL")
	}
	if _, err := v.srv.validateClientServerURL(req.URL.String()); err != nil {
		return nil, err
	}
	return v.base.RoundTrip(req)
}

// safeTransport wraps the server's (possibly telemetry-instrumented)
// transport with outbound URL validation.
func (s *Server) safeTransport() http.RoundTripper {
	base := s.transport
	if base == nil {
		base = http.DefaultTransport
	}
	return &validatingRoundTripper{base: base, srv: s}
}

// checkRedirect validates each redirect target used by outbound Immich/OAuth
// calls so a malicious or compromised upstream cannot bounce the server into
// a private address.
func (s *Server) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if _, err := s.validateClientServerURL(req.URL.String()); err != nil {
		return fmt.Errorf("refusing redirect: %w", err)
	}
	return nil
}

// httpClient returns a client with the global timeout and this server's
// redirect policy. The global client stays untouched so tests and other
// callers are not surprised.
func (s *Server) httpClient() *http.Client {
	c := *immichHTTPClient
	c.CheckRedirect = s.checkRedirect
	c.Transport = s.safeTransport()
	return &c
}
