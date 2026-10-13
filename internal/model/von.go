package model

// von is a non-autoregressive System 1 model and a drop-in for TypeSafe's Jev
// /v1/systemone protocol. It sends state as a bare JSON string and defaults to
// an 8192-token input budget.
//
// Language selection is server-side: the von checkpoint is chosen at server
// start (e.g. VON_MODEL=multilingual). archivist does not forward a lang field
// on the wire for von; the language field in Capabilities reflects the server's
// configured default ("en" unless overridden).
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
