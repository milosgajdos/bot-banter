package jet

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go/jetstream"
)

type Reader struct {
	cons    jetstream.Consumer
	subject string
}

func (r Reader) Read(ctx context.Context, prompts chan string) error {
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
