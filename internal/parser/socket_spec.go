package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "socket",
		Kind:    spec.KindKeyword,
		Summary: "WebSocket endpoint for bidirectional real-time messaging.",
		Description: "A `socket` opens a WebSocket connection at the given path. Client " +
			"events dispatch to `on connect`, `on message`, and `on disconnect` handlers. " +
			"Use `broadcast to :<room>` from any action or socket handler to push to all " +
			"connected clients in a room.",
		Syntax: "socket <path> [requires <clause>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"requires", "on", "broadcast"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Chat room socket",
				Code: `socket /chat/:room requires auth
  on connect
    query: INSERT INTO chat_member (room_id, user_id) VALUES (:room, :current_user_id)
  on message
    query: INSERT INTO message (room_id, user_id, text) VALUES (:room, :current_user_id, :text)`,
			},
		},
		SeeAlso: []string{"stream", "broadcast", "on"},
	})
}
