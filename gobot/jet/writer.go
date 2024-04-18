package jet

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go/jetstream"
)

type Writer struct {
	stream  jetstream.JetStream
	subject string
}

func (w Writer) Write(ctx context.Context, chunks chan []byte, doneTTS chan struct{}) error {
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
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-doneTTS:
					_, err := w.stream.Publish(ctx, w.subject, msg)
					if err != nil {
						return err
					}
					// reset the msg slice instead of reallocating
					msg = msg[:0]
				}
				break
			}
			msg = append(msg, chunk...)
		}
	}
}
