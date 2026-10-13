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
	systemOnePath = "/v1/systemone"
	// defaultTimeout covers first-inference model compilation (OpenVINO/CUDA can
	// take 2-3 minutes on cold start); subsequent requests are much faster.
	defaultTimeout = 5 * time.Minute
	// singleQuestionKey is the question name used for the single-question
	// convenience methods (Choice/Noul/Score).
	singleQuestionKey = "q"
)

// Transport is the shared HTTP client for the /v1/systemone wire protocol. It
// handles only marshalling, HTTP, status-error mapping, and reading the raw
// response body. Answer decoding is delegated to the responseDecoder provided
// by each model — Transport never interprets the response JSON itself.
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

// requestExtras holds optional laya-only request fields. von ignores them
// because they are emitted with omitempty.
type requestExtras struct {
	lang       string
	maxLen     int
	headMaxLen int
	device     string
}

// ask sends a single-question request, delegates JSON decoding to the provided
// responseDecoder, and returns the one answer for singleQuestionKey.
func (t *Transport) ask(ctx context.Context, state State, q wireQuestion, extra requestExtras, dec responseDecoder) (wireAnswer, error) {
	body := wireRequest{
		Model:      t.model,
		State:      t.encoder.encode(state),
		Questions:  map[string]wireQuestion{singleQuestionKey: q},
		Lang:       extra.lang,
		MaxLen:     extra.maxLen,
		HeadMaxLen: extra.headMaxLen,
		Device:     extra.device,
	}

	raw, headers, err := t.post(ctx, body)
	if err != nil {
		return wireAnswer{}, err
	}

	answers, _, truncated, err := dec.decode(raw, headers)
	if err != nil {
		return wireAnswer{}, err
	}

	ans, ok := answers[singleQuestionKey]
	if !ok {
		return wireAnswer{}, fmt.Errorf("%w: response missing answer for question %q", ErrModelUnavailable, singleQuestionKey)
	}
	if truncated != nil && *truncated {
		return ans, ErrStateTruncated
	}
	return ans, nil
}

// post marshals, sends, and returns the raw response body and headers.
// It maps HTTP status codes to typed errors but never interprets the body.
func (t *Transport) post(ctx context.Context, body wireRequest) ([]byte, http.Header, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+systemOnePath, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	res, err := t.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrModelUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	if err := statusError(res, raw); err != nil {
		return nil, nil, err
	}

	return raw, res.Header, nil
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
