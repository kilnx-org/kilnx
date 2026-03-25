package runtime

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

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
	defer room.mu.RUnlock()
	for conn := range room.clients {
		writeWSFrame(conn, message)
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

	// Origin validation (#11 fix)
	origin := r.Header.Get("Origin")
	if origin != "" {
		host := r.Host
		if !strings.Contains(origin, host) {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	roomName := r.URL.Path
	if room, ok := pathParams["room"]; ok {
		roomName = room
	}

	room := getRoom(roomName)
	room.add(conn)
	defer room.remove(conn)

	fmt.Printf("  ws connected: %s (%s)\n", roomName, conn.RemoteAddr())

	// Execute on connect
	if len(sock.OnConnect) > 0 {
		s.executeNodes(sock.OnConnect, pathParams)
	}

	// Read loop
	reader := bufio.NewReader(conn)
	for {
		msg, err := readWSFrame(reader)
		if err != nil {
			break
		}

		if len(sock.OnMessage) > 0 {
			params := make(map[string]string)
			for k, v := range pathParams {
				params[k] = v
			}
			params["body"] = string(msg)
			params["message"] = string(msg)

			s.executeNodes(sock.OnMessage, params)

			// Broadcast the message to the room
			room.broadcast(msg)
		}
	}

	fmt.Printf("  ws disconnected: %s (%s)\n", roomName, conn.RemoteAddr())

	if len(sock.OnDisconnect) > 0 {
		s.executeNodes(sock.OnDisconnect, pathParams)
	}
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

const maxWSFrameSize = 1 << 20 // 1 MB max frame size (#4 fix)

func readWSFrame(reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := reader.Read(header); err != nil {
		return nil, err
	}

	masked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7F)

	if length == 126 {
		ext := make([]byte, 2)
		if _, err := reader.Read(ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := reader.Read(ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}

	if length > maxWSFrameSize {
		return nil, fmt.Errorf("frame too large: %d bytes", length)
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := reader.Read(maskKey); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := reader.Read(payload); err != nil {
		return nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	// Check opcode: 0x8 = close
	opcode := header[0] & 0x0F
	if opcode == 0x8 {
		return nil, fmt.Errorf("close frame")
	}

	return payload, nil
}

func writeWSFrame(conn net.Conn, data []byte) {
	frame := make([]byte, 0, 2+len(data))
	frame = append(frame, 0x81) // text frame, FIN

	if len(data) < 126 {
		frame = append(frame, byte(len(data)))
	} else if len(data) < 65536 {
		frame = append(frame, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(len(data)))
		frame = append(frame, ext...)
	}

	frame = append(frame, data...)
	conn.Write(frame)
}
