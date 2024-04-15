package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

func LLMStream(ctx context.Context, llm *ollama.LLM, seedPrompt string, prompts chan string, chunks chan []byte, errCh chan error) {
	log.Println("launching LLM stream")
	defer log.Println("done streaming LLM")
	chat := NewHistory(historySize)
	chat.Add(seedPrompt)

	fmt.Println("Seed prompt: ", seedPrompt)

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
