package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	systemOnePath  = "/v1/systemone"
	defaultTimeout = 60 * time.Second
	// singleQuestionKey is the question name used for the single-question
	// convenience methods (Choice/Noul/Score).
	singleQuestionKey = "q"
)

// Transport is the shared HTTP client for the /v1/systemone wire protocol. Both
// von and laya embed one; only the encoder, base URL, model id and auth differ.
type Transport struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
	encoder    stateEncoder
}

// transportOptions configures a Transport. Zero values fall back to defaults.
type transportOptions struct {
	baseURL    string
	apiKey     string
	model      string
	encoder    stateEncoder
	httpClient *http.Client
}

func newTransport(opts transportOptions) *Transport {
	client := opts.httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &Transport{
		httpClient: client,
		baseURL:    strings.TrimRight(opts.baseURL, "/"),
		apiKey:     opts.apiKey,
		model:      opts.model,
		encoder:    opts.encoder,
	}
}

// ask sends a single-question request and returns the one answer. Multi-question
// batching is intentionally omitted here: the pipeline asks one question at a
// time, which keeps answer-decoding and error handling simple.
func (t *Transport) ask(ctx context.Context, state State, q wireQuestion, extra requestExtras) (wireAnswer, error) {
	body := wireRequest{
		Model:      t.model,
		State:      t.encoder.encode(state),
		Questions:  map[string]wireQuestion{singleQuestionKey: q},
		Lang:       extra.lang,
		MaxLen:     extra.maxLen,
		HeadMaxLen: extra.headMaxLen,
		Device:     extra.device,
	}

	resp, err := t.post(ctx, body)
	if err != nil {
		return wireAnswer{}, err
	}
	ans, ok := resp.Answers[singleQuestionKey]
	if !ok {
		return wireAnswer{}, fmt.Errorf("%w: response missing answer for question %q", ErrModelUnavailable, singleQuestionKey)
	}
	if resp.Truncation != nil && *resp.Truncation {
		return ans, ErrStateTruncated
	}
	return ans, nil
}

// requestExtras holds optional laya-only request fields. von ignores them
// because they are emitted with omitempty.
type requestExtras struct {
	lang       string
	maxLen     int
	headMaxLen int
	device     string
}

// post marshals, sends, and decodes a request, mapping HTTP status and the two
// answer shapes (von top-level, laya nested) into a single wireResponse.
func (t *Transport) post(ctx context.Context, body wireRequest) (wireResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return wireResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+systemOnePath, bytes.NewReader(payload))
	if err != nil {
		return wireResponse{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	res, err := t.httpClient.Do(req)
	if err != nil {
		return wireResponse{}, fmt.Errorf("%w: %v", ErrModelUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return wireResponse{}, fmt.Errorf("read response: %w", err)
	}

	if err := statusError(res, raw); err != nil {
		return wireResponse{}, err
	}

	resp, err := decodeResponse(raw)
	if err != nil {
		return wireResponse{}, err
	}
	// Trust the server's truncation header even when the JSON omits the field.
	if res.Header.Get("X-Von-Truncated") != "" {
		truncated := true
		resp.Truncation = &truncated
	}
	return resp, nil
}

// statusError maps non-2xx responses to typed errors.
func statusError(res *http.Response, raw []byte) error {
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		return nil
	case res.StatusCode == http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", ErrOverflowRefused, strings.TrimSpace(string(raw)))
	default:
		return fmt.Errorf("%w: status %d: %s", ErrModelUnavailable, res.StatusCode, strings.TrimSpace(string(raw)))
	}
}

// decodeResponse reconciles the two answer shapes. If "answers" is present
// (laya) it is used directly; otherwise answers are decoded from the top-level
// object (von), excluding the reserved usage/truncation keys.
func decodeResponse(raw []byte) (wireResponse, error) {
	var resp wireResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return wireResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if len(resp.Answers) > 0 {
		return resp, nil
	}

	// von shape: answers live at the top level next to usage/truncation.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return wireResponse{}, fmt.Errorf("decode top-level answers: %w", err)
	}
	answers := make(map[string]wireAnswer)
	for key, val := range top {
		if key == "usage" || key == "truncation" {
			continue
		}
		var a wireAnswer
		if err := json.Unmarshal(val, &a); err != nil {
			// Not an answer object (e.g. a scalar metadata field); skip it.
			continue
		}
		answers[key] = a
	}
	resp.Answers = answers
	return resp, nil
}
