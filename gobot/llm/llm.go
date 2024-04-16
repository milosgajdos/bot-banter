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

func Stream(ctx context.Context, llm *ollama.LLM, c Config, prompts chan string, chunks chan []byte, errCh chan error) {
	log.Println("launching LLM stream")
	defer log.Println("done streaming LLM")
	chat := NewHistory(int(c.HistSize))
	chat.Add(c.SeedPrompt)

	fmt.Println("Seed prompt: ", c.SeedPrompt)

	for {
		select {
		case <-ctx.Done():
			return
		case prompt := <-prompts:
			chat.Add(prompt)
			_, err := llms.GenerateFromSinglePrompt(ctx, llm, chat.String(),
				llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
					select {
					case <-ctx.Done():
						return nil
					case chunks <- chunk:
						return nil
					}
				}))
			if err != nil {
				select {
				case <-ctx.Done():
				case errCh <- err:
				}
				return
			}
		}
	}
}
