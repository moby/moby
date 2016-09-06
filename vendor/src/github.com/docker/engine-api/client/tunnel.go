// +build experimental

package client

import (
	"fmt"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

// Tunnel returns the WebSocket
func (cli *Client) Tunnel(ctx context.Context, port int) (*websocket.Conn, error) {
	config, err := websocket.NewConfig(
		fmt.Sprintf("/tunnel/ws?port=%d", port),
		"http://localhost",
	)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate websocket config: %v", err)
	}
	conn, err := dial(cli.proto, cli.addr, cli.transport.TLSConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to dial: %v", err)
	}
	ws, err := websocket.NewClient(config, conn)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate websocket client: %v", err)
	}
	return ws, nil
}
