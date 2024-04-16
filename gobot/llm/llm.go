package llm

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

type Config struct {
	ModelName  string
	HistSize   uint
	SeedPrompt string
}

type LLM struct {
	model      *ollama.LLM
	seedPrompt string
	histSize   uint
}

func New(c Config) (*LLM, error) {
	model, err := ollama.New(ollama.WithModel(c.ModelName))
	if err != nil {
		return nil, err
	}
	return &LLM{
		model:      model,
		seedPrompt: c.SeedPrompt,
		histSize:   c.HistSize,
	}, nil
}

func Stream(ctx context.Context, llm *LLM, prompts chan string, chunks chan []byte) error {
	log.Println("launching LLM stream")
	defer log.Println("done streaming LLM")
	chat := NewHistory(int(llm.histSize))
	chat.Add(llm.seedPrompt)

	fmt.Println("Seed prompt: ", llm.seedPrompt)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case prompt := <-prompts:
			chat.Add(prompt)
			_, err := llms.GenerateFromSinglePrompt(ctx, llm.model, chat.String(),
				llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case chunks <- chunk:
						return nil
					}
				}))
			if err != nil {
				return err
			}
		}
	}
}
