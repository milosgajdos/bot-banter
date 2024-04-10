package main

import (
	"bufio"
	"context"
	"errors"
	"log"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

func JetStreamRead(ctx context.Context, cons jetstream.Consumer, errCh chan error, done chan struct{}) {
	defer log.Println("done reading from JetStream")
	iter, err := cons.Messages()
	if err != nil {
		errCh <- err
		return
	}
ReadStreamDone:
	for {
		select {
		case <-done:
			break ReadStreamDone
		default:
			msg, err := iter.Next()
			if err != nil {
				errCh <- err
				break
			}
			log.Printf("Received a JetStream message: %s", string(msg.Data()))
			if err := msg.Ack(); err != nil {
				errCh <- err
				break
			}
			log.Println("Lets continue reading messages from JetStream")
		}
	}
	iter.Stop()
}

func JetStreamWrite(ctx context.Context, js jetstream.JetStream, chunks chan []byte, errCh chan error, done chan struct{}) {
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-done:
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				_, err := js.Publish(ctx, "rust", msg)
				if err != nil {
					errCh <- err
				}
				log.Println("Successfully published to JetStream")
				return
			}
			msg = append(msg, chunk...)
		}
	}
}

func LLMStream(ctx context.Context, js jetstream.JetStream, llm *ollama.LLM, prompt string, errCh chan error, done chan struct{}) {
	defer log.Println("done streaming LLM")

	chunks := make(chan []byte)
	go JetStreamWrite(ctx, js, chunks, errCh, done)

	_, err := llms.GenerateFromSinglePrompt(ctx, llm, prompt,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			select {
			case <-done:
				return nil
			case chunks <- chunk:
				return nil
			}
		}))
	if err != nil {
		errCh <- err
	}
}

func main() {
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

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("failed creating a new jetstream: %v", err)
	}

	ctx := context.Background()
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "banter",
		Subjects: []string{"go", "rust"},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			log.Fatalf("failed creating stream: %v", err)
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, "banter")
		if jsErr != nil {
			log.Fatalf("failed getting JS handle: %v", jsErr)
		}
	}
	log.Println("connected to stream")

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "go",
		FilterSubject: "go",
	})
	if err != nil {
		log.Fatalf("failed creating go consumer: %v", err)
	}
	log.Println("created stream consumer")

	llm, err := ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		log.Fatal("Failed creating LLM client: ", err)
	}

	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		if err := <-errCh; err != nil {
			log.Println("error streaming: ", err)
			close(done)
		}
	}()

GameOver:
	for {
		select {
		case <-done:
			break GameOver
		default:
			reader := bufio.NewReader(os.Stdin)
			prompt, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal("Failed reading seed prompt: ", err)
			}

			ctx := context.Background()
			go LLMStream(ctx, js, llm, prompt, errCh, done)
			go JetStreamRead(ctx, cons, errCh, done)

			log.Println("Let's continue talking")
		}
	}
}
