package runtime

import (
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// Slot syntax (in a fragment body, the markers are placeholders; in a caller
// block-form call body, they are content providers):
//
//   {{slot}}                            -- default slot, self-closing
//   {{slot}}fallback{{/slot}}           -- default with fallback
//   {{slot name="header"}}              -- named slot, self-closing
//   {{slot name="header"}}fb{{/slot}}   -- named with fallback
//
// Block-form fragment call:
//
//   {{DvKanban items=people}}
//     {{slot name="header"}}Custom{{/slot}}
//     <div>{name}</div>
//   {{/DvKanban}}
//
// The closing tag uses the {{/Name}} convention. Caller body content not inside
// any {{slot ...}} block becomes the default slot.

// slotOpenRe matches {{slot}} or {{slot args}}. Used to scan for slot markers
// in both fragment HTMLContent and caller block-form bodies.
var slotOpenRe = regexp.MustCompile(`\{\{slot(?:\s+([^}]+))?\}\}`)

// forwardOpenRe matches {{forward name="X"}} markers. Used inside a fragment
// body to re-emit the outer fragment's named slot content as a named slot
// block within a nested fragment call, enabling named-to-named slot forwarding.
var forwardOpenRe = regexp.MustCompile(`\{\{forward(?:\s+([^}]+))?\}\}`)

// findFragmentBlockEnd scans content for the matching {{/name}} of an opened
// {{name args}} tag at depth 1. It counts nested same-name fragment opens so
// {{Outer}}{{Outer}}{{/Outer}}{{/Outer}} pairs correctly. Returns the inner
// body (between open and close) and the position immediately after {{/name}}.
// Returns endPos == -1 when no matching close is found (treat as self-closing).
func findFragmentBlockEnd(content, name string) (body string, endPos int) {
	depth := 1
	pos := 0
	openPrefix := "{{" + name
	closeTag := "{{/" + name + "}}"
	for pos < len(content) {
		next := strings.Index(content[pos:], "{{")
		if next < 0 {
			return "", -1
		}
		next += pos
		close := strings.Index(content[next:], "}}")
		if close < 0 {
			return "", -1
		}
		close += next + 2

		tag := content[next:close]
		if tag == closeTag {
			depth--
			if depth == 0 {
				return content[:next], close
			}
		} else if strings.HasPrefix(tag, openPrefix) {
			// Confirm this is an open of the same fragment (not a longer name
			// that happens to share the prefix, e.g. {{DvKanban2}} vs {{DvKanban}}).
			rest := tag[len(openPrefix):]
			if strings.HasPrefix(rest, " ") || rest == "}}" {
				depth++
			}
		}
		pos = close
	}
	return "", -1
}

// findSlotEnd scans for the matching {{/slot}} of an opened {{slot args}} tag,
// counting nested {{slot ...}}/{{/slot}} pairs. Returns the inner body and the
// position immediately after {{/slot}}. Returns endPos == -1 if unclosed.
func findSlotEnd(content string) (body string, endPos int) {
	depth := 1
	pos := 0
	for pos < len(content) {
		next := strings.Index(content[pos:], "{{")
		if next < 0 {
			return "", -1
		}
		next += pos
		close := strings.Index(content[next:], "}}")
		if close < 0 {
			return "", -1
		}
		close += next + 2

		inner := strings.TrimSpace(content[next+2 : close-2])
		if inner == "/slot" {
			depth--
			if depth == 0 {
				return content[:next], close
			}
		} else if inner == "slot" || strings.HasPrefix(inner, "slot ") {
			depth++
		}
		pos = close
	}
	return "", -1
}

// parseSlotName parses a slot tag's argument string and returns the value of
// `name="X"` (or empty string for the default slot).
func parseSlotName(argStr string) string {
	for _, tok := range splitArgStr(argStr) {
		eq := strings.Index(tok, "=")
		if eq < 0 {
			continue
		}
		if strings.TrimSpace(tok[:eq]) != "name" {
			continue
		}
		v := strings.TrimSpace(tok[eq+1:])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') ||
				(v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		return v
	}
	return ""
}

// extractSlots scans a caller's block-form body and separates named-slot blocks
// from default-slot content. Named slots are keyed by their `name` attribute;
// everything outside any top-level {{slot ...}}...{{/slot}} block becomes the
// default slot. Default slot content has leading/trailing whitespace trimmed.
func extractSlots(body string) (named map[string]string, def string) {
	named = make(map[string]string)
	var defBuf strings.Builder
	pos := 0
	for pos < len(body) {
		loc := slotOpenRe.FindStringIndex(body[pos:])
		if loc == nil {
			defBuf.WriteString(body[pos:])
			break
		}
		absStart := pos + loc[0]
		absEnd := pos + loc[1]
		defBuf.WriteString(body[pos:absStart])

		match := slotOpenRe.FindStringSubmatch(body[absStart:absEnd])
		argStr := ""
		if len(match) > 1 {
			argStr = match[1]
		}
		slotName := parseSlotName(argStr)

		slotBody, endRel := findSlotEnd(body[absEnd:])
		if endRel < 0 {
			// Unclosed slot tag: write the open marker verbatim and continue
			// so the default slot retains it as literal text.
			defBuf.WriteString(body[absStart:absEnd])
			pos = absEnd
			continue
		}

		if slotName != "" {
			named[slotName] = slotBody
		} else {
			defBuf.WriteString(slotBody)
		}
		pos = absEnd + endRel
	}
	return named, strings.TrimSpace(defBuf.String())
}

// substituteSlots replaces slot markers in a fragment's HTMLContent with
// caller-provided slot bodies. Marker forms accepted:
//
//	{{slot}}                       -- self-closing default
//	{{slot}}fb{{/slot}}            -- default with fallback
//	{{slot name="X"}}              -- self-closing named
//	{{slot name="X"}}fb{{/slot}}   -- named with fallback
//
// If the caller did not provide a body for a slot (e.g. self-closing form, or
// a named slot the caller did not fill), the fallback content is used. If
// there is also no fallback (self-closing slot marker), the marker is removed
// entirely.
func substituteSlots(fragHTML string, named map[string]string, def string) string {
	if !strings.Contains(fragHTML, "{{slot") {
		return fragHTML
	}
	var b strings.Builder
	pos := 0
	for pos < len(fragHTML) {
		loc := slotOpenRe.FindStringIndex(fragHTML[pos:])
		if loc == nil {
			b.WriteString(fragHTML[pos:])
			break
		}
		absStart := pos + loc[0]
		absEnd := pos + loc[1]
		b.WriteString(fragHTML[pos:absStart])

		match := slotOpenRe.FindStringSubmatch(fragHTML[absStart:absEnd])
		argStr := ""
		if len(match) > 1 {
			argStr = match[1]
		}
		slotName := parseSlotName(argStr)

		fallback, endRel := findSlotEnd(fragHTML[absEnd:])
		var consumed int
		if endRel >= 0 {
			consumed = absEnd + endRel
		} else {
			consumed = absEnd
			fallback = ""
		}

		var replacement string
		if slotName != "" {
			if val, ok := named[slotName]; ok {
				replacement = val
			} else {
				replacement = fallback
			}
		} else {
			if def != "" {
				replacement = def
			} else {
				replacement = fallback
			}
		}
		b.WriteString(replacement)
		pos = consumed
	}
	return b.String()
}

// substituteForward replaces {{forward name="X"}} markers with the outer
// fragment's named slot content wrapped as a {{slot name="X"}}...{{/slot}}
// block. This lets a wrapper fragment delegate its named slots to a nested
// fragment call: the rewritten markers are recognised by the inner call's
// extractSlots pass as named-slot blocks.
//
// {{forward}} without a name is intentionally unsupported: default-to-default
// forwarding is already covered by a bare {{slot}} marker, which the outer
// substitution pass replaces with the caller's default content (which then
// becomes the inner call's default slot content naturally).
//
// When the outer caller did not provide the named slot, the marker is stripped
// entirely so the inner fragment falls back to its own slot fallback rather
// than receiving an empty override.
func substituteForward(html string, named map[string]string) string {
	if !strings.Contains(html, "{{forward") {
		return html
	}
	return forwardOpenRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := forwardOpenRe.FindStringSubmatch(match)
		argStr := ""
		if len(parts) > 1 {
			argStr = parts[1]
		}
		name := parseSlotName(argStr)
		if name == "" {
			return ""
		}
		val, ok := named[name]
		if !ok {
			return ""
		}
		return `{{slot name="` + name + `"}}` + val + `{{/slot}}`
	})
}

// resolveFragmentBody returns the fragment's HTMLContent with slot markers
// substituted. If callerBody is empty (self-closing call form), only fallback
// content survives. Caller body may itself contain {{slot name="X"}}...{{/slot}}
// blocks marking named slots; everything else becomes default slot content.
//
// {{forward name="X"}} markers in the fragment body are also processed so that
// named slots can be forwarded into nested fragment calls.
func resolveFragmentBody(frag *parser.Page, callerBody string) string {
	var fragHTML string
	for _, node := range frag.Body {
		if node.Type == parser.NodeHTML {
			fragHTML = node.HTMLContent
			break
		}
	}
	if fragHTML == "" {
		return ""
	}
	if callerBody == "" {
		html := substituteSlots(fragHTML, nil, "")
		return substituteForward(html, nil)
	}
	named, def := extractSlots(callerBody)
	html := substituteSlots(fragHTML, named, def)
	return substituteForward(html, named)
}
