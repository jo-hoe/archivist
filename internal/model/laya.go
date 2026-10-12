package model

// laya is an encoder-based System 1 model serving the same /v1/systemone
// protocol. It wraps state in {"body": ...}, nests answers under "answers",
// supports English and multilingual checkpoints, and its HTTP server caps
// Choice at 100 options. The English checkpoint defaults to a 512-token budget.
//
// See github.com/NandhaKishorM/laya.

const (
	layaDefaultMaxStateTokens   = 512
	layaDefaultMaxChoiceOptions = 100
	layaLanguage                = "en"
)

func newLaya(cfg Config) (System1, error) {
	trans := newTransport(transportOptions{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.ModelID,
		encoder:    bodyStateEncoder{},
		httpClient: cfg.HTTPClient,
	})
	maxTokens := pick(cfg.MaxStateTokens, layaDefaultMaxStateTokens)
	lang := pickStr(cfg.Language, layaLanguage)
	return &client{
		name:  "laya",
		trans: trans,
		// laya honors max_len/lang/device request fields; forward the effective values.
		extras: requestExtras{lang: lang, maxLen: maxTokens, device: deviceHint(cfg.Device)},
		caps: Capabilities{
			MaxStateTokens:   maxTokens,
			MaxChoiceOptions: pick(cfg.MaxChoiceOptions, layaDefaultMaxChoiceOptions),
			Language:         lang,
		},
	}, nil
}
