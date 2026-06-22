package server

import (
	"fmt"
	"strconv"
	"strings"
)

// ResolveAddr resolves the listening address for the service.
// Uses the port from configAddr if it's a valid port number, otherwise uses provided port.
// The port parameter is the absolute listening port (not a base port).
//
//	configAddr ":8080" + port=8100 -> ":8100"  (configAddr port is ignored, explicit port wins)
//	configAddr "0.0.0.0" + port=8100 -> ":8100"
//	configAddr ":8080" + port=0     -> ":8080"  (falls back to configAddr port)
func ResolveAddr(configAddr string, port int) string {
	actualPort := port
	if actualPort == 0 {
		if p, err := strconv.Atoi(strings.TrimPrefix(configAddr, ":")); err == nil {
			actualPort = p
		}
	}
	return fmt.Sprintf(":%d", actualPort)
}
