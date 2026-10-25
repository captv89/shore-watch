package llm

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/genai"

	"google.golang.org/adk/v2/model"
)

type capture struct{ got *model.LLMRequest }

func (c *capture) Name() string { return "capture" }
func (c *capture) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	c.got = req
	return func(func(*model.LLMResponse, error) bool) {}
}

func TestNoThoughtsDropsThoughtParts(t *testing.T) {
	c := &capture{}
	req := &model.LLMRequest{Contents: []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "hi"}}},
		{Role: "model", Parts: []*genai.Part{{Text: "thinking", Thought: true}, {FunctionCall: &genai.FunctionCall{Name: "f"}}}},
		{Role: "model", Parts: []*genai.Part{{Text: "only a thought", Thought: true}}},
	}}
	for range (noThoughts{c}).GenerateContent(context.Background(), req, false) {
	}
	if len(c.got.Contents) != 2 || len(c.got.Contents[1].Parts) != 1 || c.got.Contents[1].Parts[0].FunctionCall == nil {
		t.Errorf("got %+v", c.got.Contents)
	}
	if len(req.Contents[1].Parts) != 2 {
		t.Error("the caller's request was changed")
	}
}
