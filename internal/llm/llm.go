// Package llm builds the model a run uses: Gemini, or an open-weights model
// on NVIDIA NIM or OpenRouter, paced under the
// provider's rate limit.
package llm

import (
	"context"
	"fmt"
	"iter"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/v3/option"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
)

// New returns the model for modelID, making at most rpm requests a minute.
func New(ctx context.Context, modelID string, rpm int) (model.LLM, error) {
	m, err := newModel(ctx, modelID)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(modelID, "gemma-") {
		m = noThoughts{m}
	}
	return &paced{LLM: m, every: time.Minute / time.Duration(rpm)}, nil
}

// noThoughts drops the model's own thought parts from the history before
// each call. Gemma on the Gemini API answers 500 "Internal error" to a
// request that carries them back (checked 2026-10-20); the
// function calls and their thought signatures stay.
type noThoughts struct{ model.LLM }

func (n noThoughts) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	out := *req
	out.Contents = make([]*genai.Content, 0, len(req.Contents))
	for _, c := range req.Contents {
		cc := *c
		cc.Parts = nil
		for _, p := range c.Parts {
			if p.Thought {
				continue
			}
			cc.Parts = append(cc.Parts, p)
		}
		if len(cc.Parts) > 0 {
			out.Contents = append(out.Contents, &cc)
		}
	}
	return n.LLM.GenerateContent(ctx, &out, stream)
}

// retryEmpty asks again when an OpenAI-compatible endpoint answers 200
// with no choices. OpenRouter's free endpoints do this under load (six runs
// stopped by it on 2026-10-26); the client retries only 429 and 5xx. Only
// the first item can carry that error, so nothing has reached the caller.
type retryEmpty struct{ model.LLM }

const emptyTries = 5

// pause is the wait before the second try; it doubles each time.
var pause = 5 * time.Second

func (r retryEmpty) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		wait := pause
		for try := 1; ; try++ {
			first, empty := true, false
			for resp, err := range r.LLM.GenerateContent(ctx, req, stream) {
				if first && err != nil && strings.Contains(err.Error(), "no choices") && try < emptyTries {
					empty = true
					break
				}
				first = false
				if !yield(resp, err) {
					return
				}
			}
			if !empty {
				return
			}
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case <-time.After(wait):
			}
			wait *= 2
		}
	}
}

// paced spaces model calls at least `every` apart.
type paced struct {
	model.LLM
	every time.Duration
	mu    sync.Mutex
	last  time.Time
}

func (p *paced) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	p.mu.Lock()
	if wait := p.every - time.Since(p.last); wait > 0 {
		time.Sleep(wait)
	}
	p.last = time.Now()
	p.mu.Unlock()
	return p.LLM.GenerateContent(ctx, req, stream)
}

// nimBaseURL is NVIDIA's OpenAI-compatible endpoint, openRouterBaseURL
// OpenRouter's.
const (
	nimBaseURL        = "https://integrate.api.nvidia.com/v1"
	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// openRouterPrefix marks a model served by OpenRouter, e.g.
// openrouter/nvidia/nemotron-3.5-super-vl:free. The prefix is ours; it is
// stripped before the call.
const openRouterPrefix = "openrouter/"

// newModel picks the provider from the model ID: openrouter/vendor/model runs
// on OpenRouter, vendor/model on NVIDIA NIM, anything else on Gemini. All
// retry 429 and 5xx answers; NIM and OpenRouter throttle bursts, so they get
// more tries.
func newModel(ctx context.Context, modelID string) (model.LLM, error) {
	if name, ok := strings.CutPrefix(modelID, openRouterPrefix); ok {
		key := os.Getenv("OPEN_ROUTER_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPEN_ROUTER_API_KEY is not set")
		}
		m, err := openaimodel.NewModel(ctx, name, &openaimodel.ClientConfig{
			APIKey: key, BaseURL: openRouterBaseURL, API: openaimodel.APIChatCompletions,
			Options: []option.RequestOption{option.WithMaxRetries(10)},
		})
		if err != nil {
			return nil, err
		}
		return retryEmpty{m}, nil
	}
	if strings.Contains(modelID, "/") {
		key := os.Getenv("NVIDIA_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("NVIDIA_API_KEY is not set")
		}
		// NIM speaks Chat Completions only, not the Responses API. ADK's
		// adapter carries token usage, including reasoning tokens.
		m, err := openaimodel.NewModel(ctx, modelID, &openaimodel.ClientConfig{
			APIKey: key, BaseURL: nimBaseURL, API: openaimodel.APIChatCompletions,
			Options: []option.RequestOption{option.WithMaxRetries(10)},
		})
		if err != nil {
			return nil, err
		}
		return retryEmpty{m}, nil
	}
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY is not set")
	}
	// The free tier allows 15 requests a minute for this model and often
	// answers 503 under load. Pace calls under the limit; retry 429, 500 and 503.
	tries, delay := int32(5), 5.0
	return gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: key,
		HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{
			Attempts: &tries, InitialDelay: &delay, HTTPStatusCodes: []int32{429, 500, 503},
		}},
	})
}
