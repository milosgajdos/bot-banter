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
	"golang.org/x/sync/errgroup"
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
	go func() {
		<-sigTrap
		log.Println("shutting down: received SIGINT...")
		cancel()
	}()

	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	jetConf := jet.Config{
		StreamURL:   url,
		StreamName:  streamName,
		DurableName: botName,
		PubSubject:  pubSubject,
		SubSubject:  subSubject,
	}
	s, err := jet.NewStream(ctx, jetConf)
	if err != nil {
		log.Fatalf("failed creating JetStream: %v", err)
	}

	llmConf := llm.Config{
		ModelName:  modelName,
		HistSize:   histSize,
		SeedPrompt: seedPrompt,
	}
	l, err := llm.New(llmConf)
	if err != nil {
		log.Fatal("failed creating an LLM client: ", err)
	}

	chunks := make(chan []byte)
	prompts := make(chan string)

	log.Println("launching workers")

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return l.Stream(ctx, prompts, chunks)
	})
	g.Go(func() error {
		return s.Reader.Read(ctx, prompts)
	})
	g.Go(func() error {
		return s.Writer.Write(ctx, chunks)
	})

	var prompt string
	fmt.Println("\nYour prompt:")
	for {
		reader := bufio.NewReader(os.Stdin)
		prompt, err = reader.ReadString('\n')
		if err != nil {
			log.Println("failed reading seed prompt: ", err)
			continue
		}
		if prompt != "" {
			break
		}
	}

	// send the prompt or exit
	select {
	case prompts <- prompt:
	case <-ctx.Done():
	}

	if err := g.Wait(); err != nil {
		if err != context.Canceled {
			log.Fatalf("encountered error: %v", err)
		}
	}
}
