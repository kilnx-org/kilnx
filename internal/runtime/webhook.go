package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// handleWebhook processes an incoming webhook POST
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request, wh parser.Webhook) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB max
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature if secret is configured
	if wh.SecretEnv != "" {
		secret := os.Getenv(wh.SecretEnv)
		if secret != "" {
			sig := r.Header.Get("X-Signature-256")
			if sig == "" {
				sig = r.Header.Get("X-Hub-Signature-256")
			}
			if sig == "" {
				sig = r.Header.Get("Stripe-Signature")
			}
			if !verifySignature(body, sig, secret) {
				fmt.Printf("  webhook %s: invalid signature\n", wh.Path)
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}
	}

	// Parse the JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Extract event type from payload
	eventType := extractEventType(payload)

	// Flatten payload to params
	params := flattenPayload(payload, "event")

	fmt.Printf("  webhook %s: event=%s\n", wh.Path, eventType)

	// Find matching event handler
	for _, event := range wh.Events {
		if event.Name == eventType || event.Name == "*" {
			s.executeNodes(event.Body, params)
			break
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func verifySignature(payload []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Handle Stripe format: t=timestamp,v1=signature
	if strings.Contains(signature, "v1=") {
		return verifyStripeSignature(payload, signature, secret)
	}

	// Handle GitHub format: sha256=hex
	signature = strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

func verifyStripeSignature(payload []byte, header, secret string) bool {
	var timestamp, sig string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			sig = kv[1]
		}
	}
	if timestamp == "" || sig == "" {
		return false
	}

	// Stripe signs: timestamp.payload
	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sig))
}

func extractEventType(payload map[string]interface{}) string {
	// Try common webhook event type fields
	for _, key := range []string{"type", "event", "event_type", "action"} {
		if val, ok := payload[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return "unknown"
}

// flattenPayload converts nested JSON to flat key-value pairs
// e.g., {"data": {"id": "123"}} -> {"event_data_id": "123"}
func flattenPayload(payload map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)
	flattenMap(payload, prefix, result)
	return result
}

func flattenMap(m map[string]interface{}, prefix string, result map[string]string) {
	for key, val := range m {
		fullKey := prefix + "_" + key
		switch v := val.(type) {
		case string:
			result[fullKey] = v
		case float64:
			result[fullKey] = fmt.Sprintf("%v", v)
		case bool:
			if v {
				result[fullKey] = "true"
			} else {
				result[fullKey] = "false"
			}
		case map[string]interface{}:
			flattenMap(v, fullKey, result)
		default:
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}
