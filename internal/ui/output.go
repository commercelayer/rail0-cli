package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

var (
	jsonMode   bool
	prettyMode bool
	quietMode  bool
	agentMode  bool
)

// SetAgent selects agent mode (--agent / RAIL0_AGENT): the output contract for a program
// driving the CLI, an AI agent above all. stdout carries exactly one JSON object per
// command, on one line — {"ok":true,"data":…} or, from FatalAgent, {"ok":false,"error":…}
// — and stderr carries only JSON Lines events ({"event":…}). No spinner, no colour, no
// prompt, no free text: everything a human-only view adds is gone, so a caller parses
// both streams without guessing which kind of line it is reading.
func SetAgent(on bool) { agentMode = on }

// IsAgent reports whether agent mode is in effect.
func IsAgent() bool { return agentMode }

// SetJSON forces plain JSON output regardless of TTY state.
func SetJSON(on bool) { jsonMode = on }

// SetPretty selects the key-value/table view (--pretty) on a TTY instead of the
// default colorized JSON.
func SetPretty(on bool) { prettyMode = on }

// SetQuiet selects minimal output (--quiet): just the primary identifier of the
// result (rail0_id / transaction_hash / id), one per line for a list — so scripts
// can capture it without a JSON parser.
func SetQuiet(on bool) { quietMode = on }

// IsQuiet reports whether --quiet is in effect. A caller that decorates its result with
// fields --quiet never prints can skip the requests that only feed them.
func IsQuiet() bool { return quietMode }

// IsMachineOutput reports whether the result will be emitted as plain JSON — --json
// is set or stdout is not a terminal — the same test RunAPI applies. A caller can add
// fields only a script needs without cluttering the terminal views.
func IsMachineOutput() bool { return agentMode || jsonMode || !isTerminal(os.Stdout) }

func isTerminal(f *os.File) bool { return term.IsTerminal(f.Fd()) }

// RunAPI executes fn and prints its result. On a TTY it shows a bubbletea
// spinner on stderr while the call is in flight, then renders to stdout:
// syntax-highlighted JSON by default, or the key-value/table view when --pretty
// is set. When stdout is piped or --json is set it skips the spinner and emits
// plain indented JSON (no ANSI), suitable for jq and shell pipelines. A nil
// result (e.g. a 204 delete) prints nothing.
func RunAPI(fn func() (any, error)) error {
	return RunAPIWithMessage("fetching…", fn)
}

// RunAPIWithMessage is RunAPI with a custom spinner label — used by the atomic
// on-chain actions (authorize/capture/refund/…) so the spinner names the work in
// progress ("Authorizing…") across the whole prepare → sign → broadcast wait,
// rather than the generic "fetching…". The spinner appears the instant the call
// starts and is cleared the moment the response arrives, right before the result
// is rendered. Behaviour is otherwise identical.
func RunAPIWithMessage(msg string, fn func() (any, error)) error {
	if agentMode {
		data, err := fn()
		if err != nil {
			return err
		}
		items, page, _ := unwrap(data)
		return printEnvelopePaged(items, page)
	}
	if quietMode {
		data, err := fn()
		if err != nil {
			return err
		}
		items, _, _ := unwrap(data)
		return printQuiet(items)
	}
	if IsMachineOutput() {
		data, err := fn()
		if err != nil {
			return err
		}
		// The array alone, as always: a script pipes it to jq, and the page is in
		// agent mode's envelope for whoever needs it.
		items, _, _ := unwrap(data)
		return printJSON(items)
	}
	stop := startSpinner(msg)
	data, err := fn()
	stop()
	if err != nil {
		return err
	}
	items, page, columns := unwrap(data)
	if prettyMode {
		err = printPretty(items, columns)
	} else {
		err = printStyled(items)
	}
	if err == nil {
		printPageHint(page)
	}
	return err
}

// PageInfo is where a page of a list sits in the whole list: what the gateway says in
// its paging headers, which the list commands used to drop (they print the array, and
// the array alone cannot say whether it is everything).
type PageInfo struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages,omitempty"`
	// HasMore says rows exist beyond this page. NextPage is the --page value that
	// fetches them, absent on the last page.
	HasMore  bool `json:"has_more"`
	NextPage int  `json:"next_page,omitempty"`
}

// Paged is a list result together with its PageInfo. A command returns it from RunAPI
// in place of the bare list; every output mode prints the list exactly as before, and
// the page goes where that mode can carry it: the "page" key of the agent envelope, a
// one-line note on a terminal when more rows exist, nowhere for --json and --quiet.
type Paged struct {
	Items any
	Page  PageInfo
}

func unwrapPaged(data any) (any, *PageInfo) {
	if p, ok := data.(Paged); ok {
		return p.Items, &p.Page
	}
	return data, nil
}

// unwrap takes a command's result apart: the rows every output mode prints, the page
// they belong to (nil for a result that is not a page of a list) and the columns the
// --pretty table shows (nil for a result that names none, see Table).
func unwrap(data any) (any, *PageInfo, []Column) {
	items, page := unwrapPaged(data)
	rows, columns := unwrapTable(items)
	return rows, page, columns
}

// printPageHint tells a person at a terminal that the list goes on, and how to get the
// rest. On stderr, so the output stays the list; silent on the last page.
func printPageHint(page *PageInfo) {
	if page == nil || !page.HasMore {
		return
	}
	hint := fmt.Sprintf("page %d of %d, %d in all", page.Page, page.TotalPages, page.Total)
	if page.NextPage > 0 {
		hint += fmt.Sprintf(" — next: --page %d", page.NextPage)
	}
	fmt.Fprintln(os.Stderr, NullStyle.Render("  "+hint))
}

// Progressf writes a progress line to stderr — but ONLY when no spinner is
// running, i.e. exactly the modes that have no other sign of life: --quiet,
// --json, or a piped stdout (CI). On a TTY the spinner already says the command
// is working, so duplicating it there would fight with the redraw.
//
// stderr, never stdout: a caller parsing the result must not have to filter
// progress out of it.
func Progressf(format string, args ...any) {
	if agentMode {
		Event("progress", map[string]any{"message": fmt.Sprintf(format, args...)})
		return
	}
	if !quietMode && !jsonMode && isTerminal(os.Stdout) {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Event writes one JSON Lines event to stderr in agent mode — {"event":name, …fields} —
// and nothing otherwise: a caller that has a human sentence for the same moment uses
// Notice instead. The fields are the facts (an address, a duration in seconds), never a
// secret: stderr ends up in an agent's context and in CI logs.
func Event(name string, fields map[string]any) {
	if !agentMode {
		return
	}
	ev := make(map[string]any, len(fields)+1)
	for k, v := range fields {
		ev[k] = v
	}
	ev["event"] = name
	b, err := compactJSON(ev)
	if err != nil {
		return
	}
	fmt.Fprintln(os.Stderr, string(b))
}

// compactJSON is json.Marshal without HTML escaping: an agent reads "<name>" and "&&"
// in a message, not \u003cname\u003e. One line, no trailing newline.
func compactJSON(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n")), nil
}

// Notice tells the user something beside the result: the sentence text on stderr, or in
// agent mode the event name with fields plus that same sentence as "message", so an agent
// gets the fact as data and the wording a human would have read.
func Notice(name string, fields map[string]any, text string) {
	if agentMode {
		ev := make(map[string]any, len(fields)+1)
		for k, v := range fields {
			ev[k] = v
		}
		ev["message"] = strings.TrimSpace(text)
		Event(name, ev)
		return
	}
	fmt.Fprintln(os.Stderr, text)
}

// printEnvelope is the agent-mode result: {"ok":true,"data":…} as ONE line, so a caller
// reads exactly one line of stdout per command. A nil result (a 204) is "data":null
// rather than nothing — an empty stdout would read as a crash.
func printEnvelope(v any) error { return printEnvelopePaged(v, nil) }

// printEnvelopePaged is printEnvelope for a list: the same line, with the list's place
// in the whole as "page" beside "data" — {"ok":true,"data":[…],"page":{"page":1,
// "per_page":25,"total":71,"total_pages":3,"has_more":true,"next_page":2}}. "data" keeps
// its shape, so a caller that reads only it is unaffected.
func printEnvelopePaged(v any, page *PageInfo) error {
	b, err := compactJSON(struct {
		OK   bool      `json:"ok"`
		Data any       `json:"data"`
		Page *PageInfo `json:"page,omitempty"`
	}{true, v, page})
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

// PrintEnvelope prints the agent-mode success envelope for a caller that runs before
// the output mode is applied (help, which cobra serves ahead of every command hook).
func PrintEnvelope(v any) error { return printEnvelope(v) }

// PrintJSON prints v as plain indented JSON, for the same early caller when --json was
// asked for outside agent mode.
func PrintJSON(v any) error { return printJSON(v) }

// FatalAgent prints the agent-mode failure envelope — {"ok":false,"error":…, …} built by
// the caller — on stdout, where the success envelope would have gone, and exits with
// code. stdout rather than stderr: one stream holds the outcome whatever it was, and the
// exit code says which.
func FatalAgent(payload map[string]any, code int) {
	// "ok" first, like the success envelope, so a caller can branch on the line's start.
	b, err := compactJSON(payload)
	if err != nil || len(b) < 2 {
		b = []byte(`{"error":{"code":"cli_error","message":"could not encode the error"}}`)
	}
	fmt.Println(`{"ok":false,` + string(b[1:]))
	os.Exit(code)
}

// startSpinner draws an animated spinner on stderr and returns a stop function
// that halts it and clears the line. The first frame is written synchronously,
// so the spinner is on screen the instant the command starts working; a
// goroutine then advances it until stop() is called (when the response arrives).
// stderr is used so the rendered result on stdout stays clean for pipelines.
func startSpinner(msg string) func() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	fmt.Fprintf(os.Stderr, "\r%s %s", SpinStyle.Render(frames[0]), msg)

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-done:
				fmt.Fprint(os.Stderr, "\r\x1b[K") // clear the spinner line
				return
			case <-t.C:
				i = (i + 1) % len(frames)
				fmt.Fprintf(os.Stderr, "\r%s %s", SpinStyle.Render(frames[i]), msg)
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

// printJSON emits plain, indented JSON with no colour — the machine-readable
// form used when piped or when --json is passed.
func printJSON(v any) error {
	if v == nil {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

// printQuiet emits only the result's primary identifier (--quiet), so a script can
// capture it with `$(...)` and no JSON parser. For a list it prints one id per
// line. Falls back to plain JSON for a shape with no recognisable id.
func printQuiet(v any) error {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	// Bare array FIRST: every list command returns resp.Data (the naked slice),
	// not the paginated envelope. printQuiet only tried the envelope's "data"
	// key, so the map unmarshal failed on an array, the whole branch was
	// skipped, and `payments list -q` fell through to printJSON — the full
	// payload instead of one id per line, on every list in the CLI. (#101)
	var arr []any
	if json.Unmarshal(b, &arr) == nil {
		ids := make([]string, 0, len(arr))
		for _, item := range arr {
			if id := quietID(item); id != "" {
				ids = append(ids, id)
			}
		}
		// A non-empty list whose items carry no recognisable id (e.g. the
		// chains catalog) keeps the JSON fallback below — printing nothing
		// would hide the payload entirely.
		if len(ids) > 0 || len(arr) == 0 {
			for _, id := range ids {
				fmt.Println(id)
			}
			return nil
		}
	}
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		if data, ok := m["data"].([]any); ok { // paginated envelope → one id per line
			for _, item := range data {
				if id := quietID(item); id != "" {
					fmt.Println(id)
				}
			}
			return nil
		}
		// A one-time secret outranks every id. `webhooks create` and `rotate-secret`
		// return the shared secret exactly once — the gateway keeps only a hash — so
		// printing the id alone threw away the one value no command can show again,
		// and after a rotation the old secret is already dead. The id is recoverable
		// (`webhooks list`); the secret is not.
		if secret, ok := m["shared_secret"].(string); ok && secret != "" {
			fmt.Println(secret)
			return nil
		}
		if id := quietID(m); id != "" {
			fmt.Println(id)
			return nil
		}
	}
	return printJSON(v) // no id to extract — don't hide the payload
}

// quietID picks the most useful identifier from a decoded object, in priority
// order: the logical rail0_id, then a transaction hash, then a bare id.
func quietID(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range []string{"rail0_id", "transaction_hash", "id"} {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// printStyled prints v as colorized, indented JSON-like text for a terminal. It
// marshals v (a typed SDK value) to JSON, decodes it into a generic tree, then
// walks the tree emitting that syntax with the package styles applied to keys
// and scalar values. Object keys are emitted unquoted (colour sets them apart),
// so the result is intentionally not strict JSON — use printJSON (piped /
// --json) for that. Keys are ordered by sortFields (identity first, timestamps
// last) for a stable, readable layout rather than Go's map iteration order.
func printStyled(v any) error {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	var tree any
	if err := json.Unmarshal(b, &tree); err != nil {
		// Not a JSON object/array/scalar we can colorize — fall back to raw bytes.
		fmt.Println(string(b))
		return nil
	}
	fmt.Println(renderJSON(tree, 0, ""))
	return nil
}

// indentUnit matches encoding/json's two-space indentation so the colorized and
// plain (--json) forms line up visually.
const indentUnit = "  "

// renderJSON returns the colorized JSON text for a decoded value. depth drives
// indentation of nested objects/arrays; key is the field name this value sits
// under (empty at the root or inside arrays) and lets scalars pick a
// context-aware style — notably status values via StatusStyle.
func renderJSON(v any, depth int, key string) string {
	switch t := v.(type) {
	case nil:
		return NullStyle.Render("null")
	case bool:
		if t {
			return TrueStyle.Render("true")
		}
		return FalseStyle.Render("false")
	case float64:
		return NumberStyle.Render(numStr(t))
	case string:
		return renderString(t, key)
	case []any:
		return renderArray(t, depth, key)
	case map[string]any:
		return renderObject(t, depth)
	default:
		// Should not happen for json.Unmarshal output; encode defensively.
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// renderObject renders an object across multiple lines with sorted keys. Keys
// are emitted WITHOUT surrounding quotes (relying on colour to set them apart)
// for readability — so this colorized view is JSON-like, not strict JSON. The
// machine-readable strict-JSON form is printJSON (piped / --json).
func renderObject(m map[string]any, depth int) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortFields(keys)

	inner := strings.Repeat(indentUnit, depth+1)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		b.WriteString(inner)
		b.WriteString(KeyStyle.Render(k))
		b.WriteString(": ")
		b.WriteString(renderJSON(m[k], depth+1, k))
		if i < len(keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte('}')
	return b.String()
}

// renderArray renders a JSON array across multiple lines. key is threaded to the
// elements so a field like "transactions" still colors each element's status.
func renderArray(arr []any, depth int, key string) string {
	if len(arr) == 0 {
		return "[]"
	}
	inner := strings.Repeat(indentUnit, depth+1)
	var b strings.Builder
	b.WriteString("[\n")
	for i, item := range arr {
		b.WriteString(inner)
		b.WriteString(renderJSON(item, depth+1, key))
		if i < len(arr)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteByte(']')
	return b.String()
}

// renderString returns a quoted, colorized JSON string. A "status" field uses
// the state-aware StatusStyle; 0x-prefixed values (addresses, hashes) use
// HexStyle; everything else uses the plain string colour.
func renderString(s, key string) string {
	q := quote(s)
	switch {
	case key == "status":
		return StatusStyle(s).Render(q)
	case strings.HasPrefix(s, "0x"):
		return HexStyle.Render(q)
	default:
		return StringStyle.Render(q)
	}
}

// quote renders s as a JSON string literal (with surrounding quotes and proper
// escaping) using encoding/json, so embedded quotes/newlines stay valid.
func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return string(b)
}

// numStr formats a JSON number the way encoding/json would: whole values that
// fit in an int64 print without a decimal point or exponent (e.g. chain ids),
// the rest use the shortest round-tripping float form.
func numStr(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// fieldPriority lists the identity / human-name fields that should lead an
// object view.
var fieldPriority = map[string]int{
	"id": 0, "slug": 1, "name": 2, "label": 3, "symbol": 4, "address": 5, "title": 6,
}

// fieldGroup buckets a field for ordering: identity/name fields first (0),
// timestamps (…_at, e.g. created_at/updated_at) always last (2), everything
// else in between (1).
func fieldGroup(name string) int {
	if _, ok := fieldPriority[name]; ok {
		return 0
	}
	if strings.HasSuffix(name, "_at") {
		return 2
	}
	return 1
}

// sortFields orders field names so id and slug/name/label lead, timestamps trail
// (consistently across every resource), and the rest sit alphabetically between.
func sortFields(names []string) {
	sort.SliceStable(names, func(i, j int) bool {
		gi, gj := fieldGroup(names[i]), fieldGroup(names[j])
		if gi != gj {
			return gi < gj
		}
		if gi == 0 { // identity group: by defined priority
			return fieldPriority[names[i]] < fieldPriority[names[j]]
		}
		return names[i] < names[j] // middle & timestamps: alphabetical
	})
}

// PrintStyled is exported for testing.
func PrintStyled(v any) error { return printStyled(v) }

// Fatal prints a styled error to stderr and exits with code 1.
func Fatal(err error) { FatalCode(err, 1) }

// FatalCode prints a styled error to stderr and exits with the given code, so
// scripts can distinguish error classes (auth vs not-found vs rejected vs …).
func FatalCode(err error, code int) {
	if isTerminal(os.Stderr) {
		mark := lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true).Render("✗")
		body := ErrStyle.Render(err.Error())
		fmt.Fprintf(os.Stderr, "\n  %s  %s\n\n", mark, body)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	os.Exit(code)
}
