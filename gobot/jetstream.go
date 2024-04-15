package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func handleError(ctx context.Context, err error, errCh chan error) {
	log.Printf("error: %v", err)
	select {
	case <-ctx.Done():
	case errCh <- err:
	}
}

func NewJetStream(url string) (jetstream.JetStream, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to NATS: %v", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed creating a new stream: %v", err)
	}

	return js, nil
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

func JetStreamReader(ctx context.Context, cons jetstream.Consumer, prompts chan string, errCh chan error) {
	log.Println("launching JetStream Writer")
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
				// NOTE: we are handling ErrMsgIteratorClosed
				// because the only way the iterator is closed
				// is if we do it when the context is cancelled.
				if err == jetstream.ErrMsgIteratorClosed {
					return
				}
				handleError(ctx, err, errCh)
				return
			}
			fmt.Printf("\n[Q]: %s\n", string(msg.Data()))
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
	log.Println("launching JetStream Reader")
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-ctx.Done():
			return
		case chunk := <-chunks:
			if len(chunk) == 0 {
				fmt.Printf("\n[A]: %s\n", string(msg))
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
