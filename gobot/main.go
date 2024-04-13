package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

const (
	modelName     = "llama2"
	streamName    = "banter"
	botName       = "gobot"
	botSubSubject = "go"
	botPubName    = "rustbot"
	botPubSubject = "rust"
	historySize   = 50
)

func handleError(ctx context.Context, err error, errCh chan error) {
	log.Printf("error: %v", err)
	select {
	case <-ctx.Done():
	case errCh <- err:
	}
}

func JetStreamReader(ctx context.Context, cons jetstream.Consumer, prompts chan string, errCh chan error) {
	log.Println("launched JetStream Writer")
	defer log.Println("done reading from JetStream")
	iter, err := cons.Messages()
	if err != nil {
		handleError(ctx, err, errCh)
		return
	}
	go func() {
		<-ctx.Done()
		iter.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := iter.Next()
			if err != nil {
				// NOTE: we are handling this error like this
				// because the only way the iterator is closed
				// is if we do it when the context is cancelled.
				if err == jetstream.ErrMsgIteratorClosed {
					return
				}
				handleError(ctx, err, errCh)
				return
			}
			fmt.Printf("\n[%s]: %s", botPubName, string(msg.Data()))
			if err := msg.Ack(); err != nil {
				handleError(ctx, err, errCh)
				return
			}
			select {
			case <-ctx.Done():
				return
			case prompts <- string(msg.Data()):
			}
		}
	}
}

func JetStreamWriter(ctx context.Context, js jetstream.JetStream, chunks chan []byte, errCh chan error) {
	log.Println("launched JetStream Reader")
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-ctx.Done():
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				fmt.Printf("\n[%s]: %s", botName, string(msg))
				_, err := js.Publish(ctx, botPubSubject, msg)
				if err != nil {
					handleError(ctx, err, errCh)
					return
				}
				// reset the msg slice instead of reallocating
				msg = msg[:0]
				break
			}
			msg = append(msg, chunk...)
		}
	}
}

func JetStream(ctx context.Context, js jetstream.JetStream, prompts chan string, chunks chan []byte, errCh chan error) {
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{botSubSubject, botPubSubject},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			handleError(ctx, err, errCh)
			return
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, streamName)
		if jsErr != nil {
			handleError(ctx, err, errCh)
			return
		}
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       botName,
		FilterSubject: botSubSubject,
	})
	if err != nil {
		handleError(ctx, err, errCh)
		return
	}

	go JetStreamWriter(ctx, js, chunks, errCh)
	go JetStreamReader(ctx, cons, prompts, errCh)
}

func LLMStream(ctx context.Context, llm *ollama.LLM, prompts chan string, chunks chan []byte, errCh chan error) {
	log.Println("launched LLM stream")
	defer log.Println("done streaming LLM")
	chat := NewHistory(historySize)
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
				handleError(ctx, err, errCh)
				return
			}
		}
	}
}

func main() {
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

	nc, err := nats.Connect(url)
	if err != nil {
		log.Fatalf("failed connecting to NATS: %v", err)
	}
	go func() {
		<-ctx.Done()
		if err := nc.Drain(); err != nil {
			log.Printf("error draining NATS: %v", err)
		}
	}()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("failed creating a new stream: %v", err)
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
			log.Println("stopping: got interrupt")
		case <-ctx.Done():
			log.Println("stopping: context cancelled")
		case err := <-errCh:
			if err != nil {
				log.Printf("stopping due to error: %v", err)
			}
		}
		cancel()
	}()

	log.Printf("launching %s workers", botName)

	go LLMStream(ctx, llm, prompts, chunks, errCh)
	go JetStream(ctx, js, prompts, chunks, errCh)

	log.Println("enter your prompt")
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
