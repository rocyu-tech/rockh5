package snowflake

import (
        "sync"
        "time"
)

const (
        // Epoch is the custom epoch (2024-01-01 00:00:00 UTC)
        epoch int64 = 1704067200000

        nodeBits  uint8 = 10
        seqBits   uint8 = 12

        maxNodeID   int64 = -1 ^ (-1 << nodeBits)
        maxSeq      int64 = -1 ^ (-1 << seqBits)
        nodeShift   uint8 = seqBits
        timeShift   uint8 = seqBits + nodeBits
)

// Generator generates Snowflake IDs
type Generator struct {
        mu        sync.Mutex
        nodeID    int64
        sequence  int64
        lastTime  int64
}

var (
        generator *Generator
        initOnce  sync.Once
)

// Init initializes the snowflake generator with the given node ID.
// Thread-safe: only the first call has any effect; subsequent calls are no-ops.
func Init(nodeID int64) {
        initOnce.Do(func() {
                if nodeID < 0 || nodeID > maxNodeID {
                        panic("invalid node ID")
                }
                generator = &Generator{
                        nodeID: nodeID,
                }
        })
}

// NextID generates a new unique ID.
// Panics with a clear message if Init() was not called.
func NextID() int64 {
        if generator == nil {
                panic("snowflake: NextID called before Init()")
        }
        return generator.nextID()
}

// MustInit panics if nodeID is invalid (convenience for main functions)
func MustInit(nodeID int64) {
        Init(nodeID)
}

func (g *Generator) nextID() int64 {
        g.mu.Lock()
        defer g.mu.Unlock()

        now := time.Now().UnixMilli() - epoch

        if now == g.lastTime {
                g.sequence = (g.sequence + 1) & maxSeq
                if g.sequence == 0 {
                        // Sequence overflow, wait for next millisecond
                        for now <= g.lastTime {
                                now = time.Now().UnixMilli() - epoch
                        }
                }
        } else {
                g.sequence = 0
        }

        g.lastTime = now

        id := (now << timeShift) | (g.nodeID << nodeShift) | g.sequence
        return id
}
