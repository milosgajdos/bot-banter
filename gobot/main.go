package main

import (
	"bufio"
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

const (
	streamName  = "banter"
	consName    = "gobot"
	goSubject   = "go"
	rustSubject = "rust"
	historySize = 50
)

func handleError(ctx context.Context, err error, errCh chan error) {
	log.Printf("error: %v", err)
	select {
	case <-ctx.Done():
	case errCh <- err:
	}
}

func JetStreamReader(ctx context.Context, cons jetstream.Consumer, prompts chan string, errCh chan error) {
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
	// NOTE: we probably dont need this
	// if there is an error, we cancel
	// the context which will unblock the
	// goroutine just above.
	defer iter.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := iter.Next()
			if err != nil {
				handleError(ctx, err, errCh)
				return
			}
			log.Printf("received a JetStream message: %s", string(msg.Data()))
			if err := msg.Ack(); err != nil {
				handleError(ctx, err, errCh)
				return
			}
			select {
			case <-ctx.Done():
				return
			case prompts <- string(msg.Data()):
				log.Println("ent a new prompt to LLM: ", string(msg.Data()))
			}
		}
	}
}

func JetStreamWriter(ctx context.Context, js jetstream.JetStream, chunks chan []byte, errCh chan error) {
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-ctx.Done():
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				log.Println("publishing message to JetStream:", string(msg))
				_, err := js.Publish(ctx, rustSubject, msg)
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
		Subjects: []string{goSubject, rustSubject},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			log.Printf("failed creating stream: %v", err)
			handleError(ctx, err, errCh)
			return
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, streamName)
		if jsErr != nil {
			log.Printf("failed getting stream %s handle: %v", streamName, jsErr)
			handleError(ctx, err, errCh)
			return
		}
	}
	log.Printf("connected to stream: %s", streamName)

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consName,
		FilterSubject: goSubject,
	})
	if err != nil {
		log.Printf("failed creating consumer: %v", err)
		handleError(ctx, err, errCh)
		return
	}
	log.Println("created stream consumer")

	go JetStreamWriter(ctx, js, chunks, errCh)
	go JetStreamReader(ctx, cons, prompts, errCh)
}

func LLMStream(ctx context.Context, llm *ollama.LLM, prompts chan string, chunks chan []byte, errCh chan error) {
	defer log.Println("done streaming LLM")
	chat := NewHistory(historySize)
	for {
		select {
		case <-ctx.Done():
			return
		case prompt := <-prompts:
			log.Println("received LLM prompt:", prompt)
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
			log.Println("done streaming LLM")
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()

	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	nc, err := nats.Connect(url)
	if err != nil {
		log.Fatalf("failed connecting to nats: %v", err)
	}
	// nolint:errcheck
	defer nc.Drain()

	log.Println("connecting to NATS")

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("failed creating a new stream: %v", err)
	}

	log.Println("connecting to ollama")

	llm, err := ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		log.Fatal("failed creating LLM client: ", err)
	}

	errCh := make(chan error, 1)
	chunks := make(chan []byte)
	prompts := make(chan string)

	go func() {
		select {
		case <-c:
			log.Println("stopping: got interrupt")
		case <-ctx.Done():
			log.Println("stopping: context cancelled")
		case err := <-errCh:
			if err != nil {
				log.Printf("stopping due to error: %v", err)
			}
		}
		log.Println("cancelling context")
		cancel()
	}()

	log.Println("launching workers")

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
