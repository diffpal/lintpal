package decisions

import "github.com/diffpal/lintpal/internal/apps/lintpal/jev"

type codecKind uint8

const (
	sharedCodecKind codecKind = iota
	openAICodecKind
)

type codec interface {
	encode(jev.Request) ([]byte, error)
	decode([]byte, jev.Request) (jev.Response, error)
}
