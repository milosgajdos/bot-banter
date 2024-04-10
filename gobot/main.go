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

func handleError(err error, errCh chan error, done chan struct{}) {
	select {
	case <-done:
	case errCh <- err:
	}
}

func JetStreamReader(_ context.Context, cons jetstream.Consumer, prompts chan string, errCh chan error, done chan struct{}) {
	defer log.Println("done reading from JetStream")
	iter, err := cons.Messages()
	if err != nil {
		handleError(err, errCh, done)
		return
	}
	defer iter.Stop()

	for {
		select {
		case <-done:
			return
		default:
			msg, err := iter.Next()
			if err != nil {
				handleError(err, errCh, done)
				return
			}
			log.Printf("Received a JetStream message: %s", string(msg.Data()))
			if err := msg.Ack(); err != nil {
				handleError(err, errCh, done)
				return
			}
			select {
			case <-done:
				return
			case prompts <- string(msg.Data()):
				log.Println("sending a new prompt to LLM: ", string(msg.Data()))
			}
		}
	}
}

func JetStreamWriter(ctx context.Context, js jetstream.JetStream, prompts chan string, chunks chan []byte, errCh chan error, done chan struct{}) {
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-done:
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				log.Println("publishing message to JetStream:", string(msg))
				_, err := js.Publish(ctx, "rust", msg)
				if err != nil {
					handleError(err, errCh, done)
					return
				}
				log.Println("Successfully published to JetStream")
				// reset the msg slice instead of reallocating
				msg = msg[:0]
				break
			}
			msg = append(msg, chunk...)
		}
	}
}

func JetStream(ctx context.Context, js jetstream.JetStream, prompts chan string, chunks chan []byte, errCh chan error, done chan struct{}) {
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "banter",
		Subjects: []string{"go", "rust"},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			log.Printf("failed creating stream: %v", err)
			handleError(err, errCh, done)
			return
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, "banter")
		if jsErr != nil {
			log.Printf("failed getting JS handle: %v", jsErr)
			handleError(err, errCh, done)
			return
		}
	}
	log.Println("connected to stream")

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "go",
		FilterSubject: "go",
	})
	if err != nil {
		log.Printf("failed creating go consumer: %v", err)
		handleError(err, errCh, done)
		return
	}
	log.Println("created gobot stream consumer")

	go JetStreamWriter(ctx, js, prompts, chunks, errCh, done)
	go JetStreamReader(ctx, cons, prompts, errCh, done)
}

func LLMStream(ctx context.Context, llm *ollama.LLM, prompts chan string, chunks chan []byte, errCh chan error, done chan struct{}) {
	defer log.Println("done streaming LLM")
	chat := NewHistory(10)
	for {
		select {
		case <-done:
			return
		case prompt := <-prompts:
			log.Println("received LLM prompt:", prompt)
			chat.Add(prompt)
			_, err := llms.GenerateFromSinglePrompt(ctx, llm, chat.String(),
				llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
					select {
					case <-done:
						return nil
					case chunks <- chunk:
						return nil
					}
				}))
			log.Println("done streaming LLM")
			if err != nil {
				handleError(err, errCh, done)
				return
			}
		}
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

	llm, err := ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		log.Fatal("Failed creating LLM client: ", err)
	}

	errCh := make(chan error, 1)
	done := make(chan struct{})
	chunks := make(chan []byte)
	prompts := make(chan string)

	ctx := context.Background()
	go LLMStream(ctx, llm, prompts, chunks, errCh, done)
	go JetStream(ctx, js, prompts, chunks, errCh, done)

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
			select {
			case prompts <- prompt:
			case <-done:
				break GameOver
			}
			log.Println("Let's continue talking")
		}
	}
}
