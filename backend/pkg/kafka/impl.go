package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// ───── Producer Implementation ────────────────────────────────────────────────

// KafkaProducer is a real Kafka producer backed by segmentio/kafka-go.
type KafkaProducer struct {
	writer *kafkago.Writer
}

// NewKafkaProducer creates a new KafkaProducer connected to the given brokers.
// The producer uses synchronous writes for simplicity.
func NewKafkaProducer(brokers []string) *KafkaProducer {
	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               &kafkago.LeastBytes{},
		RequiredAcks:           kafkago.RequireOne,
		AllowAutoTopicCreation: true,
		WriteTimeout:           10 * time.Second,
		ReadTimeout:            10 * time.Second,
		BatchTimeout:           10 * time.Millisecond,
	}
	return &KafkaProducer{writer: w}
}

// Publish sends an event to the given topic.
func (p *KafkaProducer) Publish(ctx context.Context, topic, key string, event *Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	msg := kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: raw,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}
	return nil
}

// Close shuts down the producer gracefully.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// ───── Consumer Implementation ────────────────────────────────────────────────

type topicHandler struct {
	topic   string
	handler Handler
	reader  *kafkago.Reader
}

// KafkaConsumer is a real Kafka consumer backed by segmentio/kafka-go.
// Each topic subscription gets its own reader (consumer group member).
type KafkaConsumer struct {
	brokers []string
	group   string
	logger  *slog.Logger

	mu       sync.Mutex
	handlers []*topicHandler
}

// NewKafkaConsumer creates a new KafkaConsumer.
func NewKafkaConsumer(brokers []string, group string, logger *slog.Logger) *KafkaConsumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &KafkaConsumer{
		brokers: brokers,
		group:   group,
		logger:  logger,
	}
}

// Subscribe registers a handler for a specific topic.
// Must be called before Start.
func (c *KafkaConsumer) Subscribe(topic string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        c.brokers,
		Topic:          topic,
		GroupID:        c.group,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.LastOffset,
		Logger:         kafkago.LoggerFunc(func(msg string, args ...interface{}) {}),
		ErrorLogger:    kafkago.LoggerFunc(func(msg string, args ...interface{}) {}),
	})

	c.handlers = append(c.handlers, &topicHandler{
		topic:   topic,
		handler: handler,
		reader:  r,
	})
}

// Start begins consuming messages for all subscribed topics.
// Blocks until ctx is cancelled.
func (c *KafkaConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	handlers := make([]*topicHandler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.Unlock()

	if len(handlers) == 0 {
		<-ctx.Done()
		return ctx.Err()
	}

	var wg sync.WaitGroup
	for _, th := range handlers {
		wg.Add(1)
		go func(th *topicHandler) {
			defer wg.Done()
			c.consume(ctx, th)
		}(th)
	}

	wg.Wait()
	return ctx.Err()
}

func (c *KafkaConsumer) consume(ctx context.Context, th *topicHandler) {
	for {
		m, err := th.reader.FetchMessage(ctx)
		if err != nil {
			if err == context.Canceled || err == io.EOF {
				return
			}
			c.logger.Error("kafka fetch error",
				slog.String("topic", th.topic),
				slog.Any("error", err),
			)
			time.Sleep(time.Second)
			continue
		}

		var event Event
		if err := json.Unmarshal(m.Value, &event); err != nil {
			c.logger.Error("kafka unmarshal error",
				slog.String("topic", th.topic),
				slog.Any("error", err),
			)
			_ = th.reader.CommitMessages(ctx, m)
			continue
		}

		if err := th.handler(ctx, &event); err != nil {
			c.logger.Error("kafka handler error",
				slog.String("topic", th.topic),
				slog.String("event_type", event.EventType),
				slog.Any("error", err),
			)
		}

		if err := th.reader.CommitMessages(ctx, m); err != nil {
			c.logger.Error("kafka commit error",
				slog.String("topic", th.topic),
				slog.Any("error", err),
			)
		}
	}
}

// Close gracefully shuts down all readers.
func (c *KafkaConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var firstErr error
	for _, th := range c.handlers {
		if err := th.reader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ───── No-Op implementations (for testing / local dev without Kafka) ─────────

// NoopProducer discards all events silently.
type NoopProducer struct{}

func NewNoopProducer() *NoopProducer                                                    { return &NoopProducer{} }
func (NoopProducer) Publish(_ context.Context, _, _ string, _ *Event) error             { return nil }
func (NoopProducer) Close() error                                                        { return nil }

// NoopConsumer ignores all subscriptions and returns immediately on Start.
type NoopConsumer struct{}

func NewNoopConsumer() *NoopConsumer                              { return &NoopConsumer{} }
func (NoopConsumer) Subscribe(_ string, _ Handler)               {}
func (NoopConsumer) Start(ctx context.Context) error             { <-ctx.Done(); return ctx.Err() }
func (NoopConsumer) Close() error                                { return nil }
