package utils

import (
	"fmt"
	"net"
	"strconv"
)

func VerifyPortAvailable(host string, port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, portNum))
	if err != nil {
		return fmt.Errorf("port %s is not available: %w", port, err)
	}
	if closeErr := ln.Close(); closeErr != nil {
		return fmt.Errorf("failed to close listener: %w", closeErr)
	}
	return nil
}
