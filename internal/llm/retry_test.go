package llm

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// flaky answers "no choices" fails times, then a real response, as
// OpenRouter's free endpoints did on 2026-10-26.
type flaky struct {
	fails, calls int
}

func (f *flaky) Name() string { return "flaky" }

func (f *flaky) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		f.calls++
		if f.calls <= f.fails {
			yield(nil, errors.New("openai: response included no choices"))
			return
		}
		yield(&model.LLMResponse{Content: genai.NewContentFromText("ok", genai.RoleModel)}, nil)
	}
}

func run(m model.LLM) (string, error) {
	for r, err := range m.GenerateContent(context.Background(), &model.LLMRequest{}, false) {
		if err != nil {
			return "", err
		}
		return r.Content.Parts[0].Text, nil
	}
	return "", errors.New("no response")
}

func TestRetryEmpty(t *testing.T) {
	pause = time.Millisecond
	f := &flaky{fails: 2}
	if got, err := run(retryEmpty{f}); err != nil || got != "ok" || f.calls != 3 {
		t.Errorf("got %q, %v after %d calls; want ok after 3", got, err, f.calls)
	}
	f = &flaky{fails: 10}
	if _, err := run(retryEmpty{f}); err == nil || f.calls != emptyTries {
		t.Errorf("err %v after %d calls; want the error after %d", err, f.calls, emptyTries)
	}
}
