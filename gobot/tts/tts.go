package tts

import (
	"context"
	"io"
	"log"

	"github.com/milosgajdos/go-playht"
)

const (
	MaxTTSBufferSize   = 1000
	defaultQuality     = playht.Low
	defaultOutput      = playht.Mp3
	defaultSpeed       = 1.0
	defaultSampleRate  = 24000
	defaultVoiceEngine = playht.PlayHTv2Turbo
)

type Config struct {
	VoiceID      string
	VoiceEngine  playht.VoiceEngine
	Quality      playht.Quality
	OutputFormat playht.OutputFormat
	Speed        float32
	SampleRate   int32
}

func DefaultConfig() *Config {
	return &Config{
		VoiceID:      "",
		Quality:      defaultQuality,
		OutputFormat: defaultOutput,
		Speed:        defaultSpeed,
		SampleRate:   defaultSampleRate,
		VoiceEngine:  defaultVoiceEngine,
	}
}

type TTS struct {
	client *playht.Client
	config Config
}

func New(c Config) (*TTS, error) {
	client := playht.NewClient()
	return &TTS{
		client: client,
		config: c,
	}, nil
}

func (t *TTS) flush(ctx context.Context, pw *io.PipeWriter, buf *Buffer, req *playht.CreateTTSStreamReq, done chan struct{}) error {
	req.Text = buf.String()
	err := t.client.TTSStream(ctx, pw, req)
	if err != nil {
		return err
	}
	buf.Reset()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case done <- struct{}{}:
	}
	return nil
}

func (t *TTS) Stream(ctx context.Context, pw *io.PipeWriter, chunks <-chan []byte, done chan struct{}) error {
	log.Println("launching TTS stream")
	defer log.Println("done streaming TTS")
	defer pw.Close()

	buf := NewFixedSizeBuffer(MaxTTSBufferSize)
	req := &playht.CreateTTSStreamReq{
		Voice:        t.config.VoiceID,
		Quality:      t.config.Quality,
		OutputFormat: t.config.OutputFormat,
		Speed:        t.config.Speed,
		SampleRate:   t.config.SampleRate,
		VoiceEngine:  t.config.VoiceEngine,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk := <-chunks:
			if len(chunk) == 0 {
				if err := t.flush(ctx, pw, buf, req, done); err != nil {
					return err
				}
				continue
			}
			n, err := buf.Write(chunk)
			if err != nil {
				if err == ErrBufferFull {
					if err := t.flush(ctx, pw, buf, req, done); err != nil {
						return err
					}
					// NOTE: flush resets the buffer and we need
					// to write the remaining chunks to it which
					// have not fitted into the buffer on the last write.
					if _, err := buf.Write(chunk[n:]); err != nil {
						return err
					}
					continue
				}
				return err
			}
		}
	}
}
