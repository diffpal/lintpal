// Package systemone implements the typed Jev System One HTTP transport.
package systemone

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var ErrInvalidEndpoint = errors.New("invalid System One endpoint")

const typeSafeBase = "https://api.typesafe.ai"
const openRouterBase = "https://openrouter.ai/api"

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Endpoint binds a destination to one token source. Its fields are private so
// repository-controlled configuration cannot retarget a preset credential.
type Endpoint struct {
	base     *url.URL
	tokenEnv string
}

// TypeSafe uses the fixed native System One destination and token source.
func TypeSafe() Endpoint {
	base, _ := url.Parse(typeSafeBase)
	return Endpoint{base: base, tokenEnv: "TYPESAFE_API_KEY"}
}

// OpenRouter uses its TypeSafe-compatible System One destination.
func OpenRouter() Endpoint {
	base, _ := url.Parse(openRouterBase)
	return Endpoint{base: base, tokenEnv: "OPENROUTER_API_KEY"}
}

// TrustedCustom constructs a custom endpoint from process-trusted settings.
// Callers must not pass repository-controlled URLs or token env names here.
// An empty tokenEnv creates an uncredentialed endpoint.
func TrustedCustom(baseURL, tokenEnv string) (Endpoint, error) {
	if tokenEnv == "TYPESAFE_API_KEY" || tokenEnv == "OPENROUTER_API_KEY" ||
		(tokenEnv != "" && !envName.MatchString(tokenEnv)) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	base, err := url.Parse(baseURL)
	if err != nil || base == nil || base.Opaque != "" || base.User != nil || base.Host == "" ||
		base.RawQuery != "" || base.Fragment != "" || base.RawFragment != "" || base.RawPath != "" ||
		strings.Contains(base.Path, "..") || strings.HasSuffix(base.Path, "/v1/systemone") {
		return Endpoint{}, ErrInvalidEndpoint
	}
	if base.Scheme != "https" && (base.Scheme != "http" || !loopbackHost(base.Hostname())) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return Endpoint{base: base, tokenEnv: tokenEnv}, nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (e Endpoint) target() (*url.URL, error) {
	if e.base == nil {
		return nil, ErrInvalidEndpoint
	}
	target := *e.base
	target.Path = strings.TrimRight(target.Path, "/") + "/v1/systemone"
	return &target, nil
}

func (e Endpoint) token() string {
	if e.tokenEnv == "" {
		return ""
	}
	return os.Getenv(e.tokenEnv)
}

func secureClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}
