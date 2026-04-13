package runtimev1

type RuntimeConfig struct {
	APIBaseURL string     `json:"apiBaseUrl,omitempty"`
	Auth       AuthConfig `json:"auth"`
}

type AuthConfig struct {
	Issuer                 string   `json:"issuer,omitempty"`
	ClientID               string   `json:"clientId,omitempty"`
	Scopes                 []string `json:"scopes,omitempty"`
	RedirectPath           string   `json:"redirectPath,omitempty"`
	PostLogoutRedirectPath string   `json:"postLogoutRedirectPath,omitempty"`
}
