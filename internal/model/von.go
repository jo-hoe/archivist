package model

// von is a non-autoregressive System 1 model and a drop-in for TypeSafe's Jev
// /v1/systemone protocol. It sends state as a bare JSON string, is English-only,
// and defaults to an 8192-token input budget.
//
// See github.com/wfzyx/von.

const (
	vonDefaultMaxStateTokens   = 8192
	vonDefaultMaxChoiceOptions = 255
	vonLanguage                = "en"
)

func newVon(cfg Config) (System1, error) {
	trans := newTransport(transportOptions{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.ModelID,
		encoder:    stringStateEncoder{},
		httpClient: cfg.HTTPClient,
	})
	return &client{
		name:  "von",
		trans: trans,
		caps: Capabilities{
			MaxStateTokens:   pick(cfg.MaxStateTokens, vonDefaultMaxStateTokens),
			MaxChoiceOptions: pick(cfg.MaxChoiceOptions, vonDefaultMaxChoiceOptions),
			Language:         pickStr(cfg.Language, vonLanguage),
		},
	}, nil
}
