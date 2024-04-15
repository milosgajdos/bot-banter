package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"github.com/tmc/langchaingo/llms/ollama"
)

const (
	historySize   = 50
	modelName     = "llama2"
	streamName    = "banter"
	botName       = "gobot"
	botSubSubject = "go"
	botPubSubject = "rust"
)

var (
	seedPrompt string
)

func init() {
	flag.StringVar(&seedPrompt, "seed-prompt", defaultSeedPrompt, "seed prompt")
}

func main() {
	flag.Parse()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	sigTrap := make(chan os.Signal, 1)
	signal.Notify(sigTrap, os.Interrupt)
	defer func() {
		signal.Stop(sigTrap)
		cancel()
	}()

	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	jet, err := NewJetStream(url)
	if err != nil {
		log.Fatalf("failed creating JetStream: %v", err)
	}

	llm, err := ollama.New(ollama.WithModel(modelName))
	if err != nil {
		log.Fatal("failed creating an LLM client: ", err)
	}

	errCh := make(chan error, 1)
	chunks := make(chan []byte)
	prompts := make(chan string)

	go func() {
		select {
		case <-sigTrap:
			log.Println("shutting down: received SIGINT...")
		case <-ctx.Done():
			log.Println("shutting down: context cancelled")
		case err := <-errCh:
			if err != nil {
				log.Printf("shutting down, encountered error: %v", err)
			}
		}
		cancel()
	}()

	log.Printf("launching %s workers", botName)

	go LLMStream(ctx, llm, seedPrompt, prompts, chunks, errCh)
	go JetStream(ctx, jet, prompts, chunks, errCh)

	fmt.Println("\nYour prompt:")
	reader := bufio.NewReader(os.Stdin)
	prompt, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed reading seed prompt: ", err)
	}

	// send the prompt or exit
	select {
	case prompts <- prompt:
	case <-ctx.Done():
	}

	<-ctx.Done()
}
