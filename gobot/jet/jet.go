package jet

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Config struct {
	StreamURL   string
	StreamName  string
	DurableName string
	PubSubject  string
	SubSubject  string
}

type Writer struct {
	js      jetstream.JetStream
	subject string
}

type Reader struct {
	cons    jetstream.Consumer
	subject string
}

type Stream struct {
	Writer Writer
	Reader Reader
}

func NewStream(ctx context.Context, c Config) (*Stream, error) {
	nc, err := nats.Connect(c.StreamURL)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to NATS: %v", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed creating a new stream: %v", err)
	}

	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     c.StreamName,
		Subjects: []string{c.SubSubject, c.PubSubject},
	})
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
			return nil, err
		}
		var jsErr error
		stream, jsErr = js.Stream(ctx, c.StreamName)
		if jsErr != nil {
			return nil, err
		}
	}

	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       c.DurableName,
		FilterSubject: c.SubSubject,
	})
	if err != nil {
		return nil, err
	}

	return &Stream{
		Writer: Writer{
			js:      js,
			subject: c.PubSubject,
		},
		Reader: Reader{
			cons:    cons,
			subject: c.SubSubject,
		},
	}, nil
}

func Read(ctx context.Context, r Reader, prompts chan string) error {
	log.Println("launching JetStream Writer")
	defer log.Println("done reading from JetStream")
	iter, err := r.cons.Messages()
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		iter.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := iter.Next()
			if err != nil {
				// NOTE: we are handling ErrMsgIteratorClosed
				// because the only way the iterator is closed
				// is if we do it when the context is cancelled.
				if err == jetstream.ErrMsgIteratorClosed {
					return nil
				}
				return err
			}
			fmt.Printf("\n[Q]: %s\n", string(msg.Data()))
			if err := msg.Ack(); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case prompts <- string(msg.Data()):
			}
		}
	}
}

func Write(ctx context.Context, w Writer, chunks chan []byte) error {
	log.Println("launching JetStream Reader")
	defer log.Println("done writing to JetStream")
	msg := []byte{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk := <-chunks:
			if len(chunk) == 0 {
				fmt.Printf("\n[A]: %s\n", string(msg))
				_, err := w.js.Publish(ctx, w.subject, msg)
				if err != nil {
					return err
				}
				// reset the msg slice instead of reallocating
				msg = msg[:0]
				break
			}
			msg = append(msg, chunk...)
		}
	}
}
