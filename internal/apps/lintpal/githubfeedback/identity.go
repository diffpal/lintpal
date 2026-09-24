package githubfeedback

import (
	"errors"
	"strings"
)

var ErrInvalidChannel = errors.New("invalid GitHub feedback channel")

const DefaultChannel = "lintpal"

type Identity struct{ Channel string }

func NewIdentity(channel string) (Identity, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = DefaultChannel
	}
	if len(channel) > 64 {
		return Identity{}, ErrInvalidChannel
	}
	for index, char := range channel {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || index > 0 && strings.ContainsRune("-_.", char) {
			continue
		}
		return Identity{}, ErrInvalidChannel
	}
	return Identity{Channel: channel}, nil
}

func (identity Identity) ResultMarker(headSHA string) string {
	return "<!-- lintpal:result:" + identity.Channel + " head_sha:" + strings.TrimSpace(headSHA) + " -->"
}

func (identity Identity) FindingMarker(findingID, digest string) string {
	clean := strings.NewReplacer("--", "-", "\r", " ", "\n", " ").Replace(strings.TrimSpace(findingID))
	if clean == "" || digest == "" {
		return ""
	}
	return "<!-- lintpal:finding:" + identity.Channel + " id:" + clean + " digest:" + digest + " -->"
}
