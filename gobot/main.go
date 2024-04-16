package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"nats-jet/gobot/jet"
	"nats-jet/gobot/llm"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"github.com/tmc/langchaingo/llms/ollama"
)

var (
	histSize   uint
	seedPrompt string
	modelName  string
	streamName string
	botName    string
	pubSubject string
	subSubject string
)

func init() {
	flag.UintVar(&histSize, "hist-size", defaultHistSize, "chat history size")
	flag.StringVar(&seedPrompt, "seed-prompt", defaultSeedPrompt, "seed prompt")
	flag.StringVar(&modelName, "model-name", defaultModelName, "LLM model")
	flag.StringVar(&streamName, "stream-name", defaultStreamName, "jetstream name")
	flag.StringVar(&botName, "bot-name", defaultBotName, "bot name")
	flag.StringVar(&pubSubject, "pub-subject", defaultPubSubject, "bot publish subject")
	flag.StringVar(&subSubject, "sub-subject", defaultSubSubject, "bot subscribe subject")
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

	stream, err := jet.NewStream(url)
	if err != nil {
		log.Fatalf("failed creating JetStream: %v", err)
	}

	ollm, err := ollama.New(ollama.WithModel(modelName))
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

	log.Println("launching workers")

	llmConf := llm.Config{
		ModelName:  modelName,
		HistSize:   histSize,
		SeedPrompt: seedPrompt,
	}
	go llm.Stream(ctx, ollm, llmConf, prompts, chunks, errCh)

	jetConf := jet.Config{
		StreamName:  streamName,
		DurableName: botName,
		PubSubject:  pubSubject,
		SubSubject:  subSubject,
	}
	go jet.Stream(ctx, stream, jetConf, prompts, chunks, errCh)

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
