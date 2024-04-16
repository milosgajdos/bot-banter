package jet

import (
	"context"
	"errors"
	"fmt"

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
			stream:  js,
			subject: c.PubSubject,
		},
		Reader: Reader{
			cons:    cons,
			subject: c.SubSubject,
		},
	}, nil
}
