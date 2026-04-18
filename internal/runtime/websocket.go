package runtime

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// Room manages connected WebSocket clients for a path
type Room struct {
	mu      sync.RWMutex
	clients map[net.Conn]bool
}

var (
	rooms   = make(map[string]*Room)
	roomsMu sync.RWMutex
)

func getRoom(name string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()
	if rooms[name] == nil {
		rooms[name] = &Room{clients: make(map[net.Conn]bool)}
	}
	return rooms[name]
}

func (room *Room) add(conn net.Conn) {
	room.mu.Lock()
	defer room.mu.Unlock()
	room.clients[conn] = true
}

func (room *Room) remove(conn net.Conn) {
	room.mu.Lock()
	defer room.mu.Unlock()
	delete(room.clients, conn)
}

func (room *Room) broadcast(message []byte) {
	room.mu.RLock()
	clients := make([]net.Conn, 0, len(room.clients))
	for conn := range room.clients {
		clients = append(clients, conn)
	}
	room.mu.RUnlock()

	var dead []net.Conn
	for _, conn := range clients {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := writeWSFrame(conn, message); err != nil {
			dead = append(dead, conn)
		}
		conn.SetWriteDeadline(time.Time{})
	}

	if len(dead) > 0 {
		room.mu.Lock()
		for _, conn := range dead {
			delete(room.clients, conn)
			conn.Close()
		}
		room.mu.Unlock()
	}
}

// handleSocket handles WebSocket upgrade and bidirectional communication
func (s *Server) handleSocket(w http.ResponseWriter, r *http.Request, sock parser.Socket) {
	// Auth check
	if sock.Auth {
		session := s.getSession(r)
		if session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// WebSocket handshake
	if !strings.Contains(strings.ToLower(r.Header.Get("Upgrade")), "websocket") {
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	// Origin validation: parse URL and compare host component
	origin := r.Header.Get("Origin")
	if origin != "" {
		originURL, err := url.Parse(origin)
		if err != nil || originURL.Host != r.Host {
			http.Error(w, "Forbidden: origin mismatch", http.StatusForbidden)
			return
		}
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "Missing WebSocket key", http.StatusBadRequest)
		return
	}

	accept := computeAcceptKey(key)

	// Hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		s.logger.LogError("websocket hijack failed", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Send upgrade response
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	bufrw.Flush()

	// Get room name from path params
	pathParams := matchPathParams(sock.Path, r.URL.Path)
	// Bind current_user.* (including tenant id) onto the connection's params.
	s.populateCurrentUserParams(pathParams, s.getSession(r))
	roomName := r.URL.Path
	if room, ok := pathParams["room"]; ok {
		roomName = room
	}

	room := getRoom(roomName)
	room.add(conn)
	defer room.remove(conn)

	fmt.Printf("  ws connected: %s (%s)\n", roomName, conn.RemoteAddr())

	// Start ping/pong heartbeat to detect dead connections.
	// Sends a ping every 30s. The read loop resets the read deadline on every
	// received frame (data or pong). If no frame arrives within 60s the
	// read will timeout and the connection is cleaned up.
	connDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := writeWSPing(conn); err != nil {
					return
				}
				conn.SetWriteDeadline(time.Time{})
			case <-connDone:
				return
			}
		}
	}()
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// Execute on connect and send history
	if len(sock.OnConnect) > 0 {
		// Execute on connect nodes and check for query results to send as history
		if s.db != nil {
			for _, node := range sock.OnConnect {
				if node.Type == parser.NodeQuery {
					sql, tErr := RewriteTenantSQL(node.SQL, s.tenants, pathParams)
					if tErr != nil {
						s.logger.LogError("tenant guard rejected websocket on-connect query", tErr)
						continue
					}
					rows, err := s.db.QueryRowsWithParams(sql, pathParams)
					if err == nil && len(rows) > 0 {
						// Send each row as a message to the newly connected client
						for _, row := range rows {
							// Build a simple message from row values
							var parts []string
							for _, val := range row {
								parts = append(parts, val)
							}
							msg := strings.Join(parts, " ")
							writeWSFrame(conn, []byte(msg))
						}
					}
				}
			}
		}
		_ = s.executeNodes(sock.OnConnect, pathParams)
	}

	// Read loop
	reader := bufio.NewReader(conn)
	for {
		msg, opcode, err := readWSFrame(reader)
		if err != nil {
			break
		}

		// Reset read deadline on any received frame
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Pong frame (0xA): heartbeat response, no further processing needed
		if opcode == 0xA {
			continue
		}

		if len(sock.OnMessage) > 0 {
			params := make(map[string]string)
			for k, v := range pathParams {
				params[k] = v
			}
			params["body"] = string(msg)
			params["message"] = string(msg)

			_ = s.executeNodes(sock.OnMessage, params)

			// Check for broadcast nodes
			hasBroadcast := false
			for _, node := range sock.OnMessage {
				if node.Type == parser.NodeBroadcast {
					hasBroadcast = true
					// Resolve broadcast room
					broadcastRoom := node.BroadcastRoom
					if strings.HasPrefix(broadcastRoom, ":") {
						paramName := strings.TrimPrefix(broadcastRoom, ":")
						if val, ok := params[paramName]; ok {
							broadcastRoom = val
						}
					}
					targetRoom := getRoom(broadcastRoom)

					// If a fragment reference is specified, render it
					if node.BroadcastFrag != "" {
						app := s.getApp()
						rendered := false
						for _, frag := range app.Fragments {
							fragName := strings.TrimPrefix(frag.Path, "/")
							if fragName == node.BroadcastFrag {
								// Render the fragment with the current params as context
								renderedHTML := s.renderFragmentWithParams(frag, params)
								targetRoom.broadcast([]byte(renderedHTML))
								rendered = true
								break
							}
						}
						if !rendered {
							targetRoom.broadcast(msg)
						}
					} else {
						targetRoom.broadcast(msg)
					}
				}
			}

			// Default: broadcast to the room if no explicit broadcast node
			if !hasBroadcast {
				room.broadcast(msg)
			}
		}
	}

	close(connDone)
	fmt.Printf("  ws disconnected: %s (%s)\n", roomName, conn.RemoteAddr())

	if len(sock.OnDisconnect) > 0 {
		_ = s.executeNodes(sock.OnDisconnect, pathParams)
	}
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

const maxWSFrameSize = 1 << 20 // 1 MB max frame size (#4 fix)

// readWSFrame reads a WebSocket frame and returns the payload, opcode, and any error.
func readWSFrame(reader *bufio.Reader) ([]byte, byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, 0, err
	}

	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7F)

	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return nil, 0, err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return nil, 0, err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}

	if length > maxWSFrameSize {
		return nil, 0, fmt.Errorf("frame too large: %d bytes", length)
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return nil, 0, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, 0, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	// Close frame
	if opcode == 0x8 {
		return nil, opcode, fmt.Errorf("close frame")
	}

	return payload, opcode, nil
}

// writeWSPing sends a WebSocket ping frame (opcode 0x9) with no payload.
func writeWSPing(conn net.Conn) error {
	_, err := conn.Write([]byte{0x89, 0x00}) // FIN + ping opcode, zero length
	return err
}

func writeWSFrame(conn net.Conn, data []byte) error {
	frame := make([]byte, 0, 2+len(data))
	frame = append(frame, 0x81) // text frame, FIN

	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(len(data)))
		frame = append(frame, ext...)
	} else {
		frame = append(frame, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(len(data)))
		frame = append(frame, ext...)
	}

	frame = append(frame, data...)
	_, err := conn.Write(frame)
	return err
}
