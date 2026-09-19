package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"time"
)

const statusReadTimeout = 90 * time.Second

// WatchStatus keeps a local Unix-socket subscription alive and reconnects when
// the daemon is restarted. Status updates are pushed by the daemon on change
// and by its heartbeat. If three heartbeats are missed, the daemon is treated
// as unavailable and the client reconnects.
func WatchStatus(ctx context.Context) <-chan Status {
	out := make(chan Status, 8)
	go func() {
		defer close(out)
		for ctx.Err() == nil {
			path, err := SocketPath()
			if err != nil {
				return
			}
			conn, err := net.DialTimeout("unix", path, time.Second)
			if err != nil {
				emitUnavailable(out)
				if !sleepContext(ctx, 5*time.Second) {
					return
				}
				continue
			}

			_ = conn.SetReadDeadline(time.Now().Add(statusReadTimeout))
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				var status Status
				if json.Unmarshal(scanner.Bytes(), &status) == nil {
					_ = conn.SetReadDeadline(time.Now().Add(statusReadTimeout))
					select {
					case out <- status:
					case <-ctx.Done():
						_ = conn.Close()
						return
					}
				}
			}
			_ = conn.Close()
			emitUnavailable(out)
			if !sleepContext(ctx, 5*time.Second) {
				return
			}
		}
	}()
	return out
}

func emitUnavailable(out chan<- Status) {
	select {
	case out <- Status{Daemon: "unavailable", Telegram: "unavailable"}:
	default:
	}
}
