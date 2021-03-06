package rpc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	heartbeatInterval = 30 * time.Second
)

func ipcListen(endpoint string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(endpoint), 0751); err != nil {
		return nil, err
	}
	os.Remove(endpoint)
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	os.Chmod(endpoint, 0600)
	return l, nil
}

func newIPCConnection(ctx context.Context, endpoint string) (net.Conn, error) {
	d := &net.Dialer{KeepAlive: heartbeatInterval}
	return d.DialContext(ctx, "unix", endpoint)
}
