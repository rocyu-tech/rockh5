package consistenthash

import (
        "hash/crc32"
        "sort"
        "strconv"
        "sync"
)

const defaultReplicas = 150

// HashRing implements consistent hashing with virtual nodes
type HashRing struct {
        virtualNodes int
        ring         []uint32          // sorted hash ring
        nodeMap      map[uint32]string // virtual node hash -> physical node ID
        mu           sync.RWMutex
}

// NewHashRing creates a new consistent hash ring
func NewHashRing(replicas int) *HashRing {
        if replicas <= 0 {
                replicas = defaultReplicas
        }
        return &HashRing{
                virtualNodes: replicas,
                ring:         make([]uint32, 0),
                nodeMap:      make(map[uint32]string),
        }
}

// AddNode adds a physical node to the hash ring
func (h *HashRing) AddNode(nodeID string) {
        h.mu.Lock()
        defer h.mu.Unlock()

        for i := 0; i < h.virtualNodes; i++ {
                virtualKey := nodeID + "#" + strconv.Itoa(i)
                hash := h.hashKey(virtualKey)
                h.ring = append(h.ring, hash)
                h.nodeMap[hash] = nodeID
        }
        sort.Slice(h.ring, func(i, j int) bool { return h.ring[i] < h.ring[j] })
}

// RemoveNode removes a physical node from the hash ring
func (h *HashRing) RemoveNode(nodeID string) {
        h.mu.Lock()
        defer h.mu.Unlock()

        for i := 0; i < h.virtualNodes; i++ {
                virtualKey := nodeID + "#" + strconv.Itoa(i)
                hash := h.hashKey(virtualKey)
                idx := sort.Search(len(h.ring), func(j int) bool { return h.ring[j] >= hash })
                if idx < len(h.ring) && h.ring[idx] == hash {
                        h.ring = append(h.ring[:idx], h.ring[idx+1:]...)
                        delete(h.nodeMap, hash)
                }
        }
}

// GetNode returns the node responsible for the given key
func (h *HashRing) GetNode(key string) string {
        h.mu.RLock()
        defer h.mu.RUnlock()

        if len(h.ring) == 0 {
                return ""
        }

        hash := h.hashKey(key)
        idx := sort.Search(len(h.ring), func(i int) bool { return h.ring[i] >= hash })
        if idx == len(h.ring) {
                idx = 0
        }
        return h.nodeMap[h.ring[idx]]
}

// GetNodes returns the N nodes responsible for the given key (for replication)
func (h *HashRing) GetNodes(key string, count int) []string {
        if count <= 0 {
                return nil
        }
        if count > len(h.nodeMap) {
                count = len(h.nodeMap)
        }

        h.mu.RLock()
        defer h.mu.RUnlock()

        if len(h.ring) == 0 {
                return nil
        }

        hash := h.hashKey(key)
        idx := sort.Search(len(h.ring), func(i int) bool { return h.ring[i] >= hash })
        if idx == len(h.ring) {
                idx = 0
        }

        result := make([]string, 0, count)
        seen := make(map[string]bool)
        startIdx := idx
        for len(result) < count {
                nodeID := h.nodeMap[h.ring[idx]]
                if !seen[nodeID] {
                        seen[nodeID] = true
                        result = append(result, nodeID)
                }
                idx = (idx + 1) % len(h.ring)
                if idx == startIdx {
                        break
                }
        }
        return result
}

func (h *HashRing) hashKey(key string) uint32 {
        return crc32.ChecksumIEEE([]byte(key))
}
