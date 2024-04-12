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

const (
	streamName  = "banter"
	consName    = "gobot"
	goSubject   = "go"
	rustSubject = "rust"
	historySize = 50
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
			log.Printf("received a JetStream message: %s", string(msg.Data()))
			if err := msg.Ack(); err != nil {
				handleError(err, errCh, done)
				return
			}
			select {
			case <-done:
				return
			case prompts <- string(msg.Data()):
				log.Println("ent a new prompt to LLM: ", string(msg.Data()))
			}
		}
	}
}

func JetStreamWriter(ctx context.Context, js jetstream.JetStream, chunks chan []byte, errCh chan error, done chan struct{}) {
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-done:
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				log.Println("publishing message to JetStream:", string(msg))
				_, err := js.Publish(ctx, rustSubject, msg)
				if err != nil {
					handleError(err, errCh, done)
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

func JetStream(ctx context.Context, js jetstream.JetStream, prompts chan string, chunks chan []byte, errCh chan error, done chan struct{}) {
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{goSubject, rustSubject},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			log.Printf("failed creating stream: %v", err)
			handleError(err, errCh, done)
			return
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, streamName)
		if jsErr != nil {
			log.Printf("failed getting stream %s handle: %v", streamName, jsErr)
			handleError(err, errCh, done)
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
		handleError(err, errCh, done)
		return
	}
	log.Println("created stream consumer")

	go JetStreamWriter(ctx, js, chunks, errCh, done)
	go JetStreamReader(ctx, cons, prompts, errCh, done)
}

func LLMStream(ctx context.Context, llm *ollama.LLM, prompts chan string, chunks chan []byte, errCh chan error, done chan struct{}) {
	defer log.Println("done streaming LLM")
	chat := NewHistory(historySize)
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
		log.Fatalf("failed creating a new stream: %v", err)
	}

	llm, err := ollama.New(ollama.WithModel("llama2"))
	if err != nil {
		log.Fatal("failed creating LLM client: ", err)
	}

	reader := bufio.NewReader(os.Stdin)
	prompt, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("failed reading seed prompt: ", err)
	}

	errCh := make(chan error, 1)
	done := make(chan struct{})
	chunks := make(chan []byte)
	prompts := make(chan string)

	go func() {
		if err := <-errCh; err != nil {
			log.Printf("ending chat due to error: %v", err)
			close(done)
		}
	}()

	ctx := context.Background()
	go LLMStream(ctx, llm, prompts, chunks, errCh, done)
	go JetStream(ctx, js, prompts, chunks, errCh, done)

	// send the prompt or exit
	select {
	case prompts <- prompt:
	case <-done:
	}

	<-done
}
