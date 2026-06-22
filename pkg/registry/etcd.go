package registry

import (
        "context"
        "encoding/json"
        "fmt"
        "math"
        "net"
        "path"
        "strconv"
        "strings"
        "time"

        "go.etcd.io/etcd/api/v3/mvccpb"
        clientv3 "go.etcd.io/etcd/client/v3"
        "github.com/rocyu-tech/rockgame/pkg/logger"
)

const (
        // servicePrefix is the etcd key prefix for service registration.
        // Layout: /rockgame/services/<service-name>/<instance-id> → JSON value
        servicePrefix = "/rockgame/services/"

        // routePrefix is the etcd key prefix for dynamic route configuration.
        // Layout: /rockgame/routes/<prefix> → JSON {"backend":"...", "auth":true/false}
        // Example: /rockgame/routes/api/v1/account → {"backend":"account-node","auth":true}
        routePrefix = "/rockgame/routes/"

        // defaultTTL is the lease TTL for service registration.
        defaultTTL = 10 // seconds

        // defaultKeepAliveInterval is how often to send keepalive.
        defaultKeepAliveInterval = 3 * time.Second
)

// RouteEntry represents a route configuration stored in etcd.
type RouteEntry struct {
        Prefix  string `json:"prefix"`  // path prefix, e.g. "/api/v1/account"
        Backend string `json:"backend"` // backend service name, e.g. "account-node"
        Auth    bool   `json:"auth"`    // whether JWT auth is required
}

// ServiceInstance represents a registered service instance stored in etcd.
type ServiceInstance struct {
        Name       string            `json:"name"`      // service name, e.g. "account-node"
        ID         string            `json:"id"`        // unique instance ID, e.g. "account-node-0"
        Addr       string            `json:"addr"`      // host:port, e.g. "10.0.0.1:8001"
        Port       int               `json:"port"`      // listen port
        NodeID     int               `json:"node_id"`   // node instance index
        Tags       []string          `json:"tags"`      // optional tags for filtering
        Meta       map[string]string `json:"meta"`      // key-value metadata
        Registered int64             `json:"registered"` // unix timestamp
}

// EtcdRegistry provides service registration and discovery via etcd.
type EtcdRegistry struct {
        client          *clientv3.Client
        leaseID         clientv3.LeaseID
        cancelKeepAlive context.CancelFunc
        prefix          string
}

// NewEtcdRegistry creates an EtcdRegistry connected to the given endpoints.
func NewEtcdRegistry(endpoints []string) (*EtcdRegistry, error) {
        client, err := clientv3.New(clientv3.Config{
                Endpoints:   endpoints,
                DialTimeout: 5 * time.Second,
        })
        if err != nil {
                return nil, fmt.Errorf("etcd: failed to connect to %v: %w", endpoints, err)
        }

        return &EtcdRegistry{
                client: client,
                prefix: servicePrefix,
        }, nil
}

// Register registers a service instance in etcd with a lease for auto-expiration.
func (r *EtcdRegistry) Register(instance ServiceInstance) error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        leaseResp, err := r.client.Grant(ctx, defaultTTL)
        if err != nil {
                return fmt.Errorf("etcd: failed to create lease: %w", err)
        }
        r.leaseID = leaseResp.ID

        instance.Registered = time.Now().Unix()
        data, err := json.Marshal(instance)
        if err != nil {
                return fmt.Errorf("etcd: failed to marshal instance: %w", err)
        }

        key := path.Join(r.prefix, instance.Name, instance.ID)
        _, err = r.client.Put(ctx, key, string(data), clientv3.WithLease(r.leaseID))
        if err != nil {
                return fmt.Errorf("etcd: failed to register %s: %w", key, err)
        }

        // Start keepalive goroutine
        keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())
        r.cancelKeepAlive = keepAliveCancel
        ch, err := r.client.KeepAlive(keepAliveCtx, r.leaseID)
        if err != nil {
                return fmt.Errorf("etcd: failed to start keepalive: %w", err)
        }

        go func() {
                for range ch {
                        // keepalive response consumed, lease renewed
                }
                logger.Warnf("etcd: keepalive channel closed for %s (lease expired or cancelled)", instance.ID)
        }()

        logger.Infof("etcd: registered [%s] id=%s addr=%s:%d key=%s",
                instance.Name, instance.ID, instance.Addr, instance.Port, key)

        return nil
}

// Deregister removes the service instance from etcd and revokes the lease.
func (r *EtcdRegistry) Deregister(serviceName, instanceID string) error {
        if r.cancelKeepAlive != nil {
                r.cancelKeepAlive()
        }

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        key := path.Join(r.prefix, serviceName, instanceID)
        _, err := r.client.Delete(ctx, key)
        if err != nil {
                return fmt.Errorf("etcd: failed to deregister %s: %w", key, err)
        }

        if r.leaseID != 0 {
                _, _ = r.client.Revoke(ctx, r.leaseID)
        }

        logger.Infof("etcd: deregistered [%s] id=%s", serviceName, instanceID)
        return nil
}

// DiscoverServices returns all healthy instances of a given service name.
func (r *EtcdRegistry) DiscoverServices(serviceName string, tags ...string) ([]ServiceInstance, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        prefix := path.Join(r.prefix, serviceName) + "/"
        resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
        if err != nil {
                return nil, fmt.Errorf("etcd: failed to discover %s: %w", serviceName, err)
        }

        var instances []ServiceInstance
        for _, kv := range resp.Kvs {
                var inst ServiceInstance
                if err := json.Unmarshal(kv.Value, &inst); err != nil {
                        logger.Warnf("etcd: unmarshal failed for key %s: %v", string(kv.Key), err)
                        continue
                }
                if len(tags) > 0 && !hasAnyTag(inst.Tags, tags) {
                        continue
                }
                instances = append(instances, inst)
        }

        return instances, nil
}

// DiscoverAddrs returns host:port strings for all healthy instances.
func (r *EtcdRegistry) DiscoverAddrs(serviceName string) ([]string, error) {
        instances, err := r.DiscoverServices(serviceName)
        if err != nil {
                return nil, err
        }
        addrs := make([]string, len(instances))
        for i, inst := range instances {
                addrs[i] = inst.Addr
        }
        return addrs, nil
}

// WatchServices watches a service prefix for changes and calls the callback.
// Blocks until stopCh is closed. Callback receives the current instance list.
// Uses exponential backoff on disconnection (500ms → 1s → 2s → ... → 30s cap).
func (r *EtcdRegistry) WatchServices(serviceName string, callback func([]ServiceInstance), stopCh <-chan struct{}) {
        prefix := path.Join(r.prefix, serviceName) + "/"
        var attempt int

        for {
                select {
                case <-stopCh:
                        return
                default:
                }

                watchCh := r.client.Watch(context.Background(), prefix, clientv3.WithPrefix())

                inner:
                for {
                        select {
                        case <-stopCh:
                                return
                        case watchResp, ok := <-watchCh:
                                if !ok {
                                        sleep := etcdBackoff(attempt)
                                        attempt++
                                        logger.Warnf("etcd: service watch channel closed for [%s], reconnecting in %v (attempt %d)...",
                                                serviceName, sleep, attempt)
                                        // I12: use select on stopCh instead of blocking time.Sleep
                                        select {
                                        case <-stopCh:
                                                return
                                        case <-time.After(sleep):
                                        }
                                        break inner // break to outer loop to re-create Watch
                                }

                                // Successful event — reset backoff
                                attempt = 0

                                // Collect current state after each event
                                ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
                                resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
                                cancel()

                                var instances []ServiceInstance
                                if err == nil {
                                        for _, kv := range resp.Kvs {
                                                var inst ServiceInstance
                                                if err := json.Unmarshal(kv.Value, &inst); err != nil {
                                                        logger.Warnf("etcd: watch unmarshal failed for key %s: %v", string(kv.Key), err)
                                                        continue
                                                }
                                                instances = append(instances, inst)
                                        }
                                } else {
                                        // Skip callback on Get failure — calling callback(nil) would
                                        // erroneously clear all backend targets for this service.
                                        logger.Warnf("etcd: failed to refresh [%s] after watch event: %v", serviceName, err)
                                        continue
                                }

                                if len(watchResp.Events) > 0 {
                                        for _, ev := range watchResp.Events {
                                                switch ev.Type {
                                                case mvccpb.PUT:
                                                        logger.Infof("etcd: service up [%s] key=%s", serviceName, path.Base(string(ev.Kv.Key)))
                                                case mvccpb.DELETE:
                                                        logger.Infof("etcd: service down [%s] key=%s", serviceName, path.Base(string(ev.Kv.Key)))
                                                }
                                        }
                                }

                                callback(instances)
                        }
                }
        }
}

// Close shuts down the etcd registry and revokes the lease.
func (r *EtcdRegistry) Close() error {
        if r.cancelKeepAlive != nil {
                r.cancelKeepAlive()
        }
        if r.leaseID != 0 {
                ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
                defer cancel()
                r.client.Revoke(ctx, r.leaseID)
        }
        return r.client.Close()
}

// Client returns the underlying etcd client for advanced operations.
func (r *EtcdRegistry) Client() *clientv3.Client {
        return r.client
}

// ── Route Configuration (dynamic routing via etcd) ──

// PutRoute stores a route configuration in etcd.
func (r *EtcdRegistry) PutRoute(entry RouteEntry) error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        key := routePrefix + strings.TrimLeft(entry.Prefix, "/")
        data, err := json.Marshal(entry)
        if err != nil {
                return fmt.Errorf("etcd: failed to marshal route: %w", err)
        }

        _, err = r.client.Put(ctx, key, string(data))
        if err != nil {
                return fmt.Errorf("etcd: failed to put route %s: %w", key, err)
        }
        logger.Infof("etcd: route stored: %s → %s", key, entry.Backend)
        return nil
}

// PutRoutes stores multiple route configurations in a single transaction.
func (r *EtcdRegistry) PutRoutes(entries []RouteEntry) error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        ops := make([]clientv3.Op, len(entries))
        for i, entry := range entries {
                key := routePrefix + strings.TrimLeft(entry.Prefix, "/")
                data, err := json.Marshal(entry)
                if err != nil {
                        return fmt.Errorf("etcd: failed to marshal route: %w", err)
                }
                ops[i] = clientv3.OpPut(key, string(data))
        }

        _, err := r.client.Txn(ctx).Then(ops...).Commit()
        if err != nil {
                return fmt.Errorf("etcd: failed to batch put routes: %w", err)
        }
        logger.Infof("etcd: %d routes stored in transaction", len(entries))
        return nil
}

// DeleteRoute removes a route configuration from etcd.
func (r *EtcdRegistry) DeleteRoute(prefix string) error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        key := routePrefix + strings.TrimLeft(prefix, "/")
        _, err := r.client.Delete(ctx, key)
        if err != nil {
                return fmt.Errorf("etcd: failed to delete route %s: %w", key, err)
        }
        logger.Infof("etcd: route deleted: %s", key)
        return nil
}

// GetRoutes returns all route configurations from etcd.
func (r *EtcdRegistry) GetRoutes() ([]RouteEntry, error) {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        resp, err := r.client.Get(ctx, routePrefix, clientv3.WithPrefix())
        if err != nil {
                return nil, fmt.Errorf("etcd: failed to get routes: %w", err)
        }

        var entries []RouteEntry
        for _, kv := range resp.Kvs {
                var entry RouteEntry
                if err := json.Unmarshal(kv.Value, &entry); err != nil {
                        logger.Warnf("etcd: unmarshal route failed for key %s: %v", string(kv.Key), err)
                        continue
                }
                entries = append(entries, entry)
        }
        return entries, nil
}

// WatchRoutes watches the route prefix for changes and calls the callback
// with the full route table on every change. Blocks until stopCh is closed.
// Uses exponential backoff on disconnection (500ms → 1s → 2s → ... → 30s cap).
func (r *EtcdRegistry) WatchRoutes(callback func([]RouteEntry), stopCh <-chan struct{}) {
        var attempt int

        // Fire initial load
        if entries, err := r.GetRoutes(); err == nil && len(entries) > 0 {
                callback(entries)
        }

        for {
                select {
                case <-stopCh:
                        return
                default:
                }

                watchCh := r.client.Watch(context.Background(), routePrefix, clientv3.WithPrefix())

                inner:
                for {
                        select {
                        case <-stopCh:
                                return
                        case watchResp, ok := <-watchCh:
                                if !ok {
                                        sleep := etcdBackoff(attempt)
                                        attempt++
                                        logger.Warnf("etcd: route watch channel closed, reconnecting in %v (attempt %d)...", sleep, attempt)
                                        // I12: use select on stopCh instead of blocking time.Sleep
                                        select {
                                        case <-stopCh:
                                                return
                                        case <-time.After(sleep):
                                        }
                                        break inner // break to outer loop to re-create Watch
                                }

                                // Successful event — reset backoff
                                attempt = 0

                                // Collect current state after each event
                                entries, err := r.GetRoutes()
                                if err != nil {
                                        logger.Warnf("etcd: failed to refresh routes after watch event: %v", err)
                                        continue
                                }

                                // Log individual changes
                                for _, ev := range watchResp.Events {
                                        switch ev.Type {
                                        case mvccpb.PUT:
                                                logger.Infof("etcd: route added/updated: %s", path.Base(string(ev.Kv.Key)))
                                        case mvccpb.DELETE:
                                                logger.Infof("etcd: route removed: %s", path.Base(string(ev.Kv.Key)))
                                        }
                                }

                                callback(entries)
                        }
                }
        }
}

// ── Helper functions ──

// etcdBackoff computes exponential backoff duration with jitter for reconnection.
func etcdBackoff(attempt int) time.Duration {
        const (
                maxBackoff     = 30 * time.Second
                initialBackoff = 500 * time.Millisecond
                backoffFactor  = 2.0
        )
        d := float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt))
        if d > float64(maxBackoff) {
                d = float64(maxBackoff)
        }
        // ±10% jitter to avoid thundering herd across service instances
        jitter := d * 0.1 * (float64(time.Now().UnixNano()%1000)/1000.0 - 0.5)
        return time.Duration(d + jitter)
}

// ServiceID generates a unique instance ID: name-nodeID
func ServiceID(name string, nodeID int) string {
        return fmt.Sprintf("%s-%d", name, nodeID)
}

// ExtractHostIP gets the machine's primary non-loopback IPv4 address.
func ExtractHostIP(bindAddr string) string {
        host, _, err := net.SplitHostPort(bindAddr)
        if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
                addrs, err := net.InterfaceAddrs()
                if err != nil {
                        return "127.0.0.1"
                }
                for _, addr := range addrs {
                        if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
                                if ipnet.IP.To4() != nil {
                                        return ipnet.IP.String()
                                }
                        }
                }
                return "127.0.0.1"
        }
        return host
}

// ResolveEndpoints converts a single host:port string to endpoints slice.
func ResolveEndpoints(addr string) []string {
        if addr == "" {
                return []string{"127.0.0.1:2379"}
        }
        return []string{addr}
}

// hasAnyTag checks if instance tags contain any filter tag.
func hasAnyTag(tags []string, filters []string) bool {
        for _, f := range filters {
                for _, t := range tags {
                        if strings.EqualFold(t, f) {
                                return true
                        }
                }
        }
        return false
}

// BuildAddr joins host and port into host:port string.
func BuildAddr(host string, port int) string {
        return net.JoinHostPort(host, strconv.Itoa(port))
}