package decisions

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/diffpal/lintpal/internal/apps/lintpal/jev"
)

func TestOpenAIEncodingPreservesCriteriaAndOrder(t *testing.T) {
	request := typedRequest()
	request.Model = "gpt-6-luna"
	request.State = "bounded source"
	request.Questions["n"] = jev.NoulQuestion{Instructions: "Risk?", Criteria: &jev.NoulCriteria{True: "unsafe", False: "safe"}}
	body, err := (openAICodec{}).encode(request)
	if err != nil {
		t.Fatal(err)
	}
	var wire openAIRequestWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Questions) != 3 || wire.Questions[0].Name != "c" || wire.Questions[1].Name != "n" || wire.Questions[2].Name != "s" {
		t.Fatalf("question order: %+v", wire.Questions)
	}
	if got := wire.Questions[0].Choices; len(got) != 2 || got[0].Value != "risk" || got[0].Description != "risk" || got[1].Value != "safe" {
		t.Fatalf("choices: %+v", got)
	}
	if got := wire.Questions[1].Instructions; !strings.Contains(got, "- true: unsafe") || !strings.Contains(got, "- false: safe") {
		t.Fatalf("predicate criteria lost: %q", got)
	}
	if got := wire.Questions[2].Levels; len(got) != 2 || got[0].Label != "0" || got[0].Description != "low" || got[1].Label != "1" {
		t.Fatalf("levels: %+v", got)
	}
	bodyAgain, err := (openAICodec{}).encode(request)
	if err != nil || string(bodyAgain) != string(body) {
		t.Fatalf("encoding is not deterministic: %v", err)
	}
}

func TestOpenAIEncodingRejectsNonTextState(t *testing.T) {
	if _, err := (openAICodec{}).encode(typedRequest()); !errors.Is(err, jev.ErrInvalidRequest) {
		t.Fatalf("non-text state error = %v", err)
	}
}

func TestOpenAICost(t *testing.T) {
	reported := 0.25
	if got := openAICost("other", 100, 5, 20, 10, nil); got != nil {
		t.Fatalf("unknown model cost = %v", *got)
	}
	if got := openAICost("gpt-6-luna", 100, 5, 20, 10, &reported); got != &reported {
		t.Fatalf("reported cost was replaced: %v", got)
	}
	got := openAICost("gpt-6-luna", 100, 5, 20, 10, nil)
	if got == nil || math.Abs(*got-0.00001095) > 1e-15 {
		t.Fatalf("calculated cost = %v", got)
	}
}

func TestOpenAIDecodeRejectsTypedChoiceValuesAndBadCacheAccounting(t *testing.T) {
	request := typedRequest()
	request.Model = "gpt-6-luna"
	request.State = "bounded source"
	for _, body := range []string{
		strings.Replace(openAIResponse, `"value":"safe"`, `"value":true`, 1),
		strings.Replace(openAIResponse, `"cached_tokens":20`, `"cached_tokens":95`, 1),
	} {
		if _, err := (openAICodec{}).decode([]byte(body), request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid response accepted: %v", err)
		}
	}
}
