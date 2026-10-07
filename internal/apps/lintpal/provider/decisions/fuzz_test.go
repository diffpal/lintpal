package decisions

import (
	"errors"
	"strings"
	"testing"
)

func FuzzResponseEnvelope(f *testing.F) {
	f.Add([]byte(validResponse))
	f.Add([]byte(`{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`))
	f.Add([]byte(`{"model":"jev-1.13.0","answers":{"n":{"type":"noul","noul":2}},"usage":{"input_tokens":1,"output_tokens":1}}`))
	f.Add([]byte(`{"raw":"provider-body-sentinel"}`))
	f.Add([]byte(strings.Repeat("x", maxResponseBytes+1)))
	request := typedRequest()
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxResponseBytes {
			return // The transport rejects this before decoding.
		}
		response, err := decodeResponse(data, request)
		if err != nil {
			if !errors.Is(err, ErrProtocol) || strings.Contains(err.Error(), "provider-body-sentinel") || len(response.Answers) != 0 {
				t.Fatal("unsafe or partial response")
			}
		}
	})
}

func FuzzOpenAIResponseEnvelope(f *testing.F) {
	f.Add([]byte(openAIResponse))
	f.Add([]byte(`{"model":"gpt-6-luna","answers":[{"type":"refusal","name":"n"}],"usage":{"input_tokens":1,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":0}}`))
	f.Add([]byte(`{"raw":"openai-provider-body-sentinel"}`))
	request := typedRequest()
	request.Model = "gpt-6-luna"
	request.State = "bounded source"
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxResponseBytes {
			return
		}
		response, err := (openAICodec{}).decode(data, request)
		if err != nil && (!errors.Is(err, ErrProtocol) || strings.Contains(err.Error(), "openai-provider-body-sentinel") || len(response.Answers) != 0) {
			t.Fatal("unsafe or partial OpenAI response")
		}
	})
}
