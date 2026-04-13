package config

type Config struct {
	App  AppConfig
	Auth AuthConfig
}

type AppConfig struct {
	Name string
	Env  string
	Port int
}

type AuthConfig struct {
	Issuer                 string
	ClientID               string
	Scopes                 []string
	RedirectPath           string
	PostLogoutRedirectPath string
}

func Default() *Config {
	return &Config{
		App: AppConfig{
			Name: "basic",
			Env:  "development",
			Port: 3000,
		},
		Auth: AuthConfig{
			Issuer:                 "http://localhost:8080",
			ClientID:               "basic-browser",
			Scopes:                 []string{"openid", "profile", "email", "offline_access"},
			RedirectPath:           "/auth/callback",
			PostLogoutRedirectPath: "/",
		},
	}
}
