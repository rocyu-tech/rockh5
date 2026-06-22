package eventbus

import (
        "context"
        "encoding/json"
        "fmt"
        "math"
        "strings"
        "sync"
        "time"

        "github.com/redis/go-redis/v9"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

// Event represents a domain event
type Event struct {
        Name      string      `json:"name"`
        Data      interface{} `json:"data"`
        Timestamp int64       `json:"timestamp"`
}

// Handler is a function that handles events
type Handler func(ctx context.Context, event *Event) error

// EventBus provides publish/subscribe event bus using Redis Streams.
// Supports both in-process dispatch and cross-service delivery via Redis Streams XREADGROUP.
type EventBus struct {
        redis      *redis.Client
        handlers   map[string][]Handler
        mu         sync.RWMutex
        streamName string
        groupName  string

        // consumer lifecycle
        running bool
        stopCh  chan struct{}
        wg      sync.WaitGroup
}

var bus *EventBus

// Init initializes the global event bus
func Init(rdb *redis.Client, streamName, groupName string) {
        bus = &EventBus{
                redis:      rdb,
                handlers:   make(map[string][]Handler),
                streamName: streamName,
                groupName:  groupName,
        }
}

// Bus returns the global event bus
func Bus() *EventBus {
        return bus
}

// Subscribe registers a handler for an event type (in-process).
func Subscribe(eventName string, handler Handler) {
        if bus == nil {
                logger.Warn("eventbus: Subscribe called before Init")
                return
        }
        bus.mu.Lock()
        defer bus.mu.Unlock()
        bus.handlers[eventName] = append(bus.handlers[eventName], handler)
}

// Publish publishes an event to the Redis Stream (XADD).
func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
        if eb.redis == nil {
                return fmt.Errorf("eventbus: redis not initialized")
        }
        data, err := json.Marshal(event)
        if err != nil {
                return fmt.Errorf("marshal event failed: %w", err)
        }
        _, err = eb.redis.XAdd(ctx, &redis.XAddArgs{
                Stream: eb.streamName,
                Values: map[string]interface{}{
                        "event": string(data),
                },
                MaxLen:    10000, // trim stream to prevent unbounded growth
                Approx:    true,
        }).Result()
        return err
}

// Emit is a convenience function to publish an event to the global bus.
func Emit(ctx context.Context, eventName string, data interface{}) error {
        if bus == nil {
                return fmt.Errorf("eventbus: not initialized")
        }
        return bus.Publish(ctx, &Event{
                Name:      eventName,
                Data:      data,
                Timestamp: time.Now().UnixMilli(),
        })
}

// Dispatch calls all registered in-process handlers for an event.
// G10: all handlers are called even if some fail; returns combined error.
func (eb *EventBus) Dispatch(ctx context.Context, event *Event) error {
        eb.mu.RLock()
        handlers := eb.handlers[event.Name]
        eb.mu.RUnlock()

        var errs []error
        for i, handler := range handlers {
                if err := handler(ctx, event); err != nil {
                        errs = append(errs, fmt.Errorf("handler[%d](%s): %w", i, event.Name, err))
                }
        }
        if len(errs) > 0 {
                return fmt.Errorf("%d handler(s) failed: %v", len(errs), errs[0])
        }
        return nil
}

// StartConsumer begins consuming events from the Redis Stream using XREADGROUP.
// Uses consumer group for at-least-once delivery. Each message is ACK'd after
// all registered in-process handlers succeed.
//
// Reconnects with exponential backoff on connection failure (500ms → 1s → 2s → ... → 30s cap).
// Must be called after Init(). Blocks briefly to launch the goroutine, then returns.
func (eb *EventBus) StartConsumer() {
        if eb.redis == nil {
                logger.Warn("eventbus: consumer not started — redis not initialized")
                return
        }
        if eb.running {
                return
        }
        eb.running = true
        eb.stopCh = make(chan struct{})
        eb.wg.Add(1)
        go eb.consumeLoop()
}

// StopConsumer stops the consumer goroutine and waits for it to finish.
func (eb *EventBus) StopConsumer() {
        if !eb.running {
                return
        }
        eb.running = false
        close(eb.stopCh)
        eb.wg.Wait()
}

// consumeLoop is the main consumer loop with exponential backoff reconnection.
func (eb *EventBus) consumeLoop() {
        defer eb.wg.Done()

        consumerName := fmt.Sprintf("%s-%d", eb.groupName, time.Now().UnixNano())
        logger.Infof("eventbus: consumer [%s] started on stream %s group %s",
                consumerName, eb.streamName, eb.groupName)

        // Create consumer group if not exists (ignore BUSYGROUP error)
        eb.ensureGroup()

        var attempt int
        for {
                if !eb.running {
                        return
                }

                // XREADGROUP: block for 5s, then retry (allows checking stopCh)
                streams, err := eb.redis.XReadGroup(context.Background(), &redis.XReadGroupArgs{
                        Group:    eb.groupName,
                        Consumer: consumerName,
                        Streams:  []string{eb.streamName, ">"}, // ">" = new messages only
                        Count:    10,
                        Block:    5 * time.Second,
                }).Result()

                if err != nil {
                        if err == redis.Nil {
                                // No new messages (timeout) — check if we should stop
                                select {
                                case <-eb.stopCh:
                                        logger.Infof("eventbus: consumer [%s] stopped", consumerName)
                                        return
                                default:
                                        continue
                                }
                        }
                        // Connection error — backoff and retry
                        sleep := backoffDuration(attempt)
                        attempt++
                        logger.Warnf("eventbus: XREADGROUP error (attempt %d): %v, retrying in %v...", attempt, err, sleep)
                        select {
                        case <-eb.stopCh:
                                logger.Infof("eventbus: consumer [%s] stopped during backoff", consumerName)
                                return
                        case <-time.After(sleep):
                                // Re-create group in case it was lost
                                eb.ensureGroup()
                                continue
                        }
                }

                // Reset backoff on successful read
                attempt = 0

                // Process messages
                for _, stream := range streams {
                        for _, msg := range stream.Messages {
                                eb.processMessage(msg.ID)
                        }
                }
        }
}

// processMessage deserializes, dispatches to in-process handlers, and ACKs.
func (eb *EventBus) processMessage(msgID string) {
        eventData, ok := msg.Values["event"]
        if !ok {
                logger.Warnf("eventbus: message %s missing 'event' field, ACK-ing", msgID)
                eb.ack(msgID)
                return
        }

        eventJSON, ok := eventData.(string)
        if !ok {
                logger.Warnf("eventbus: message %s 'event' field is not a string, ACK-ing", msgID)
                eb.ack(msgID)
                return
        }

        var event Event
        if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
                logger.Warnf("eventbus: unmarshal error for message %s: %v, ACK-ing", msgID, err)
                eb.ack(msgID)
                return
        }

        // Dispatch to in-process handlers
        ctx := context.Background()
        if err := eb.Dispatch(ctx, &event); err != nil {
                logger.Errorf("eventbus: handler error for %s (msg=%s): %v — message NOT ACK'd (will retry)", event.Name, msgID, err)
                return // Don't ACK — message will be redelivered
        }

        // All handlers succeeded — ACK the message
        eb.ack(msgID)
}

// ack acknowledges a message. Logs error but does not retry (message will be redelivered after XAUTOCLAIM).
func (eb *EventBus) ack(msgID string) {
        if err := eb.redis.XAck(context.Background(), eb.streamName, eb.groupName, msgID).Err(); err != nil {
                logger.Warnf("eventbus: XACK failed for msg %s: %v", msgID, err)
        }
}

// ensureGroup creates the consumer group if it doesn't exist.
// G9: uses strings.Contains for BUSYGROUP detection (works across Redis versions).
func (eb *EventBus) ensureGroup() {
        ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
        defer cancel()
        err := eb.redis.XGroupCreateMkStream(ctx, eb.streamName, eb.groupName, "0").Err()
        if err != nil {
                // BUSYGROUP = group already exists, which is fine.
                // G9: use strings.Contains instead of exact string match — different Redis
                // versions format the error differently (e.g. "BUSYGROUP Consumer Group name
                // already exists" vs shortened forms).
                if !strings.Contains(err.Error(), "BUSYGROUP") {
                        logger.Warnf("eventbus: XGroupCreate error: %v", err)
                }
        }
}

// backoffDuration computes exponential backoff with jitter.
func backoffDuration(attempt int) time.Duration {
        const (
                maxBackoff     = 30 * time.Second
                initialBackoff = 500 * time.Millisecond
                backoffFactor  = 2.0
        )
        d := float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt))
        if d > float64(maxBackoff) {
                d = float64(maxBackoff)
        }
        // ±10% jitter
        jitter := d * 0.1 * (float64(time.Now().UnixNano()%1000)/1000.0 - 0.5)
        return time.Duration(d + jitter)
}