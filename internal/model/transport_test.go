package model

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient spins up an httptest server returning the given status/body and
// returns a von-configured client pointed at it.
func newTestClient(t *testing.T, name string, status int, body string, hdr map[string]string) (System1, *[]wireRequest) {
	t.Helper()
	var captured []wireRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req wireRequest
		_ = json.Unmarshal(raw, &req)
		captured = append(captured, req)
		for k, v := range hdr {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	m, err := New(Config{Name: name, BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New(%s): %v", name, err)
	}
	return m, &captured
}

func TestChoiceVonTopLevelShape(t *testing.T) {
	// von returns answers at the top level, keyed by question name.
	body := `{"q":{"choice":"invoice","confidence":0.91,"probabilities":{"invoice":0.91,"insurance":0.09}},"usage":{"input_tokens":12}}`
	m, captured := newTestClient(t, "von", http.StatusOK, body, nil)

	ans, err := m.Choice(context.Background(), State{Body: "tire change invoice"}, ChoiceQuestion{
		Instructions: "pick",
		Criteria:     map[string]string{"invoice": "bills", "insurance": "policies"},
		Options:      []string{"invoice", "insurance"},
	})
	if err != nil {
		t.Fatalf("Choice: %v", err)
	}
	if ans.Choice != "invoice" || ans.Confidence != 0.91 {
		t.Fatalf("unexpected answer: %+v", ans)
	}
	// von encodes state as a bare string.
	if got := (*captured)[0].State; got != "tire change invoice" {
		t.Fatalf("von state should be a bare string, got %#v", got)
	}
}

func TestChoiceLayaNestedShape(t *testing.T) {
	// laya nests answers under "answers" and wraps state in {"body":...}.
	body := `{"answers":{"q":{"choice":"insurance","confidence":0.7,"probabilities":{"insurance":0.7,"invoice":0.3}}},"usage":{"input_tokens":5}}`
	m, captured := newTestClient(t, "laya", http.StatusOK, body, nil)

	ans, err := m.Choice(context.Background(), State{Body: "policy doc"}, ChoiceQuestion{
		Instructions: "pick",
		Criteria:     map[string]string{"invoice": "bills", "insurance": "policies"},
		Options:      []string{"invoice", "insurance"},
	})
	if err != nil {
		t.Fatalf("Choice: %v", err)
	}
	if ans.Choice != "insurance" {
		t.Fatalf("unexpected choice: %q", ans.Choice)
	}
	// laya wraps state as a map.
	m2, ok := (*captured)[0].State.(map[string]any)
	if !ok || m2["body"] != "policy doc" {
		t.Fatalf("laya state should be {body:...}, got %#v", (*captured)[0].State)
	}
}

func TestNoulGatesOnRawWhenPresent(t *testing.T) {
	// Noul reports a banded value but gating uses noul_raw.
	body := `{"q":{"noul":0.85,"noul_raw":0.62,"confidence":0.9},"usage":{"input_tokens":3}}`
	m, _ := newTestClient(t, "von", http.StatusOK, body, nil)

	ans, err := m.Noul(context.Background(), State{Body: "x"}, NoulQuestion{Instructions: "is it a tax doc?"})
	if err != nil {
		t.Fatalf("Noul: %v", err)
	}
	if ans.Noul != 0.85 || ans.NoulRaw != 0.62 {
		t.Fatalf("expected Noul=0.85 NoulRaw=0.62, got %+v", ans)
	}
}

func TestNoulRawFallsBackToNoul(t *testing.T) {
	body := `{"q":{"noul":0.4,"confidence":0.9},"usage":{"input_tokens":3}}`
	m, _ := newTestClient(t, "von", http.StatusOK, body, nil)

	ans, err := m.Noul(context.Background(), State{Body: "x"}, NoulQuestion{Instructions: "?"})
	if err != nil {
		t.Fatalf("Noul: %v", err)
	}
	if ans.NoulRaw != 0.4 {
		t.Fatalf("NoulRaw should fall back to Noul, got %v", ans.NoulRaw)
	}
}

func TestNoulSendsNoCriteria(t *testing.T) {
	// NoulQuestion has no Criteria field — descriptions go in Instructions.
	// Verify nothing is sent as criteria on the wire.
	body := `{"q":{"noul":0.9,"confidence":0.9},"usage":{"input_tokens":3}}`
	m, captured := newTestClient(t, "von", http.StatusOK, body, nil)

	_, err := m.Noul(context.Background(), State{Body: "x"}, NoulQuestion{
		Instructions: `Does the tag "finance" (Bills and invoices) apply?`,
	})
	if err != nil {
		t.Fatalf("Noul: %v", err)
	}
	q := (*captured)[0].Questions["q"]
	if q.Criteria != nil {
		t.Fatalf("noul should send no criteria, got %v", q.Criteria)
	}
}

func TestLayaNoulBodyStateNoCriteria(t *testing.T) {
	// laya uses bodyStateEncoder + nested "answers"; verify noul works with
	// no criteria (description is in Instructions).
	body := `{"answers":{"q":{"noul":0.75,"confidence":0.75}},"usage":{"input_tokens":5}}`
	m, captured := newTestClient(t, "laya", http.StatusOK, body, nil)

	_, err := m.Noul(context.Background(), State{Body: "insurance renewal"}, NoulQuestion{
		Instructions: `Does the tag "insurance" (Insurance policies and claims) apply?`,
	})
	if err != nil {
		t.Fatalf("laya Noul: %v", err)
	}
	req := (*captured)[0]
	// laya wraps state as {"body": ...}.
	stateMap, ok := req.State.(map[string]any)
	if !ok || stateMap["body"] != "insurance renewal" {
		t.Fatalf("laya state should be {body:...}, got %#v", req.State)
	}
	// no criteria sent.
	if req.Questions["q"].Criteria != nil {
		t.Fatalf("noul should send no criteria, got %v", req.Questions["q"].Criteria)
	}
}

func TestOverflowRefusedMaps422(t *testing.T) {
	m, _ := newTestClient(t, "von", http.StatusUnprocessableEntity, "context window exceeded", nil)
	_, err := m.Choice(context.Background(), State{Body: "x"}, ChoiceQuestion{
		Criteria: map[string]string{"a": "a"}, Options: []string{"a"},
	})
	if !errors.Is(err, ErrOverflowRefused) {
		t.Fatalf("expected ErrOverflowRefused, got %v", err)
	}
}

func TestTruncationHeaderSignalsTruncated(t *testing.T) {
	body := `{"q":{"choice":"a","confidence":0.5,"probabilities":{"a":1}},"usage":{"input_tokens":9}}`
	m, _ := newTestClient(t, "von", http.StatusOK, body, map[string]string{"X-Von-Truncated": "1"})
	_, err := m.Choice(context.Background(), State{Body: "x"}, ChoiceQuestion{
		Criteria: map[string]string{"a": "a"}, Options: []string{"a"},
	})
	if !errors.Is(err, ErrStateTruncated) {
		t.Fatalf("expected ErrStateTruncated, got %v", err)
	}
}

func TestEmptyChoiceRejected(t *testing.T) {
	m, _ := newTestClient(t, "von", http.StatusOK, `{}`, nil)
	_, err := m.Choice(context.Background(), State{Body: "x"}, ChoiceQuestion{})
	if !errors.Is(err, ErrEmptyChoice) {
		t.Fatalf("expected ErrEmptyChoice, got %v", err)
	}
}

func TestUnknownModel(t *testing.T) {
	_, err := New(Config{Name: "nope", BaseURL: "http://x"})
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestDefaultModelIsVon(t *testing.T) {
	m, err := New(Config{BaseURL: "http://x"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if m.Name() != "von" {
		t.Fatalf("default model should be von, got %q", m.Name())
	}
}
