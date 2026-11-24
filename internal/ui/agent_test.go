package ui

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStderr is captureStdout for stderr: agent-mode events are a contract of their
// own, read line by line by the caller.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func withAgent(t *testing.T) {
	t.Helper()
	SetAgent(true)
	t.Cleanup(func() { SetAgent(false) })
}

// The agent contract: ONE line of stdout per command, the result under "data".
func TestAgentResultIsOneEnvelopeLine(t *testing.T) {
	withAgent(t)
	out := captureStdout(t, func() {
		if err := RunAPI(func() (any, error) { return map[string]any{"rail0_id": "0xabc", "status": "authorized"}, nil }); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("want exactly one line, got %q", out)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if !env.OK || env.Data["rail0_id"] != "0xabc" {
		t.Errorf("envelope = %+v", env)
	}
}

// A 204 is still an answer: "data":null, never an empty stdout.
func TestAgentNilResultIsDataNull(t *testing.T) {
	withAgent(t)
	out := captureStdout(t, func() { _ = RunAPI(func() (any, error) { return nil, nil }) })
	if strings.TrimSpace(out) != `{"ok":true,"data":null}` {
		t.Errorf("got %q", out)
	}
}

// --quiet and --pretty set by a stored default must not leak into agent mode: the
// envelope is the only shape.
func TestAgentModeWinsOverQuietAndPretty(t *testing.T) {
	withAgent(t)
	SetQuiet(true)
	SetPretty(true)
	t.Cleanup(func() { SetQuiet(false); SetPretty(false) })
	out := captureStdout(t, func() { _ = RunAPI(func() (any, error) { return map[string]any{"id": "x"}, nil }) })
	if !strings.HasPrefix(out, `{"ok":true,"data":`) {
		t.Errorf("got %q", out)
	}
}

// stderr in agent mode is JSON Lines only: a notice carries its fields and the sentence.
func TestAgentNoticeIsAJSONEvent(t *testing.T) {
	withAgent(t)
	errOut := captureStderr(t, func() {
		Notice("signed_in", map[string]any{"address": "0xabc"}, "Signed in as 0xabc")
		Progressf("waiting… %ds elapsed", 30)
	})
	lines := strings.Split(strings.TrimSpace(errOut), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 event lines, got %q", errOut)
	}
	var ev map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatal(err)
	}
	if ev["event"] != "signed_in" || ev["address"] != "0xabc" || ev["message"] != "Signed in as 0xabc" {
		t.Errorf("event = %v", ev)
	}
	if err := json.Unmarshal([]byte(lines[1]), &ev); err != nil || ev["event"] != "progress" {
		t.Errorf("progress line = %q", lines[1])
	}
}

// Outside agent mode a notice is the plain sentence and an Event is nothing.
func TestNoticeOutsideAgentModeIsPlainText(t *testing.T) {
	errOut := captureStderr(t, func() {
		Notice("signed_in", map[string]any{"address": "0xabc"}, "Signed in as 0xabc")
		Event("dispute_reason", map[string]any{"reason": nil})
	})
	if errOut != "Signed in as 0xabc\n" {
		t.Errorf("got %q", errOut)
	}
}

// A list in agent mode: the rows under "data" as always, and where the page sits in the
// whole list under "page".
func TestAgentListCarriesItsPage(t *testing.T) {
	withAgent(t)
	out := captureStdout(t, func() {
		err := RunAPI(func() (any, error) {
			return Paged{
				Items: []string{"a", "b"},
				Page:  PageInfo{Page: 1, PerPage: 2, Total: 5, TotalPages: 3, HasMore: true, NextPage: 2},
			}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	want := `{"ok":true,"data":["a","b"],"page":{"page":1,"per_page":2,"total":5,"total_pages":3,"has_more":true,"next_page":2}}` + "\n"
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}

// The last page says so, and names no next one.
func TestAgentLastPageHasNoNext(t *testing.T) {
	withAgent(t)
	out := captureStdout(t, func() {
		_ = RunAPI(func() (any, error) {
			return Paged{Items: []string{"e"}, Page: PageInfo{Page: 3, PerPage: 2, Total: 5, TotalPages: 3}}, nil
		})
	})
	if !strings.Contains(out, `"has_more":false`) || strings.Contains(out, "next_page") {
		t.Errorf("got %q", out)
	}
}

// A result that is not a list has no "page" key at all.
func TestAgentEnvelopeWithoutAPage(t *testing.T) {
	withAgent(t)
	out := captureStdout(t, func() { _ = RunAPI(func() (any, error) { return map[string]any{"id": "x"}, nil }) })
	if strings.Contains(out, `"page"`) {
		t.Errorf("got %q", out)
	}
}

// --json (or a pipe) prints the list alone, as it always did: a script pipes it to jq.
func TestJSONModePrintsTheBareList(t *testing.T) {
	SetJSON(true)
	t.Cleanup(func() { SetJSON(false) })
	out := captureStdout(t, func() {
		_ = RunAPI(func() (any, error) {
			return Paged{Items: []string{"a"}, Page: PageInfo{Page: 1, Total: 9, HasMore: true, NextPage: 2}}, nil
		})
	})
	var rows []string
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 {
		t.Errorf("not the bare list: %v (%q)", err, out)
	}
}
