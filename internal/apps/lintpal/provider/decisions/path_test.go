package decisions

import (
	"errors"
	"testing"
)

func TestCustomPathPreservesBasePrefix(t *testing.T) {
	for _, base := range []string{"https://example.test/api", "https://example.test/api/"} {
		endpoint, err := TrustedCustomPath(base, "/v1/decisions", "MY_KEY")
		if err != nil {
			t.Fatal(err)
		}
		target, err := endpoint.target()
		if err != nil || target.String() != "https://example.test/api/v1/decisions" {
			t.Fatalf("target = %v, error = %v", target, err)
		}
	}
}

func TestCustomRejectsAmbiguousPaths(t *testing.T) {
	for _, path := range []string{"", "relative", "//other.test", "https://other.test/v1/decisions", "/a?key=x", "/a#fragment", "/../a", "/a/./b", "/a/%2e%2e/b", "/a%2fb", "/a%5cb", "/a\\b", "/a\nb", "/a b", "/a//b"} {
		if _, err := TrustedCustomPath("https://example.test/api", path, "MY_KEY"); !errors.Is(err, ErrInvalidEndpoint) {
			t.Errorf("path %q: error = %v", path, err)
		}
	}
	for _, base := range []string{"https://example.test/api/%2e%2e", "https://example.test/api/./b", "https://example.test/api//b", "https://example.test/api/v1/decisions/"} {
		if _, err := TrustedCustomPath(base, "/v1/decisions", "MY_KEY"); !errors.Is(err, ErrInvalidEndpoint) {
			t.Errorf("base %q: error = %v", base, err)
		}
	}
}
