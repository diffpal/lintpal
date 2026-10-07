// Package decisions implements the shared typed Decisions HTTP transport.
package decisions

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var ErrInvalidEndpoint = errors.New("invalid Decisions endpoint")

const typeSafeBase = "https://api.typesafe.ai"
const openRouterBase = "https://openrouter.ai/api"
const openAIBase = "https://api.openai.com"

// typeSafeInputUSDPerMillion is TypeSafe's published direct Jev list price.
const typeSafeInputUSDPerMillion = 0.042

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Endpoint binds a destination to one token source. Its fields are private so
// repository-controlled configuration cannot retarget a preset credential.
type Endpoint struct {
	base                *url.URL
	apiPath             string
	tokenEnv            string
	codecKind           codecKind
	usagePricing        bool
	inputUSDPerMillion  float64
	outputUSDPerMillion float64
}

func (e Endpoint) codec() (codec, error) {
	switch e.codecKind {
	case sharedCodecKind:
		return sharedCodec{}, nil
	case openAICodecKind:
		return openAICodec{}, nil
	default:
		return nil, ErrInvalidEndpoint
	}
}

// TypeSafe uses the fixed native System One destination and token source.
func TypeSafe() Endpoint {
	base, _ := url.Parse(typeSafeBase)
	return Endpoint{base: base, apiPath: "/v1/systemone", tokenEnv: "TYPESAFE_API_KEY",
		usagePricing: true, inputUSDPerMillion: typeSafeInputUSDPerMillion}
}

// OpenRouter uses its fixed Decisions destination and token source.
func OpenRouter() Endpoint {
	base, _ := url.Parse(openRouterBase)
	return Endpoint{base: base, apiPath: "/alpha/decisions", tokenEnv: "OPENROUTER_API_KEY"}
}

// OpenAI uses the official Decisions destination and its own wire codec.
func OpenAI() Endpoint {
	base, _ := url.Parse(openAIBase)
	return Endpoint{base: base, apiPath: "/v1/decisions", tokenEnv: "OPENAI_API_KEY", codecKind: openAICodecKind}
}

// TrustedCustom constructs a custom endpoint from process-trusted settings.
// Callers must not pass repository-controlled URLs or token env names here.
// An empty tokenEnv creates an uncredentialed endpoint.
func TrustedCustom(baseURL, tokenEnv string) (Endpoint, error) {
	return TrustedCustomPath(baseURL, "/v1/systemone", tokenEnv)
}

// TrustedCustomPath constructs a custom endpoint with a process-trusted path.
// Paths are unescaped and appended to the base prefix without URL resolution.
func TrustedCustomPath(baseURL, apiPath, tokenEnv string) (Endpoint, error) {
	if !validAPIPath(apiPath) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	if tokenEnv == "TYPESAFE_API_KEY" || tokenEnv == "OPENROUTER_API_KEY" || tokenEnv == "OPENAI_API_KEY" ||
		(tokenEnv != "" && !envName.MatchString(tokenEnv)) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	base, err := url.Parse(baseURL)
	if err != nil || base == nil || base.Opaque != "" || base.User != nil || base.Host == "" ||
		base.RawQuery != "" || base.Fragment != "" || base.RawFragment != "" || base.RawPath != "" ||
		strings.Contains(base.Path, "..") || strings.HasSuffix(strings.TrimRight(base.Path, "/"), apiPath) || !validBasePath(base.Path) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	if base.Scheme != "https" && (base.Scheme != "http" || !loopbackHost(base.Hostname())) {
		return Endpoint{}, ErrInvalidEndpoint
	}
	base.Path = strings.TrimRight(base.Path, "/")
	return Endpoint{base: base, apiPath: apiPath, tokenEnv: tokenEnv}, nil
}

// Reject escaping and normalization aliases so HTTP clients and servers agree
// on the literal destination, including the trusted base prefix.
func validAPIPath(path string) bool {
	return len(path) > 0 && len(path) <= 2048 && strings.HasPrefix(path, "/") &&
		!strings.HasPrefix(path, "//") && validBasePath(path)
}

func validBasePath(path string) bool {
	for _, r := range path {
		if r <= ' ' || r >= 127 || strings.ContainsRune("%\\?#", r) {
			return false
		}
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return !strings.Contains(path, "//")
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (e Endpoint) target() (*url.URL, error) {
	if e.base == nil || e.apiPath == "" {
		return nil, ErrInvalidEndpoint
	}
	target := *e.base
	target.Path = strings.TrimRight(target.Path, "/") + e.apiPath
	return &target, nil
}

func (e Endpoint) token() string {
	if e.tokenEnv == "" {
		return ""
	}
	return os.Getenv(e.tokenEnv)
}

func (e Endpoint) costUSD(inputTokens, outputTokens int, reported *float64) *float64 {
	if reported != nil || !e.usagePricing {
		return reported
	}
	cost := (float64(inputTokens)*e.inputUSDPerMillion + float64(outputTokens)*e.outputUSDPerMillion) / 1_000_000
	return &cost
}

func secureClient(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}
