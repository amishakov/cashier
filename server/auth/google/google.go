package google

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cashier-go/cashier/server/auth"
	"github.com/cashier-go/cashier/server/config"
	"github.com/cashier-go/cashier/server/metrics"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleapi "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (
	revokeURL = "https://accounts.google.com/o/oauth2/revoke?token=%s"
	name      = "google"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Google account.
type Config struct {
	config    *oauth2.Config
	domain    string
	whitelist map[string]bool
}

var _ auth.Provider = (*Config)(nil)

// New creates a new Google provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	uw := make(map[string]bool)
	for _, u := range c.UsersWhitelist {
		uw[u] = true
	}
	if c.ProviderOpts["domain"] == "" && len(uw) == 0 {
		return nil, errors.New("either Google Apps domain or users whitelist must be specified")
	}

	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{googleapi.UserinfoEmailScope, googleapi.UserinfoProfileScope},
		},
		domain:    c.ProviderOpts["domain"],
		whitelist: uw,
	}, nil
}

// A new oauth2 http client.
func (c *Config) newClient(ctx context.Context, token *oauth2.Token) *http.Client {
	return c.config.Client(ctx, token)
}

// Name returns the name of the provider.
func (c *Config) Name() string {
	return name
}

// Valid validates the oauth token.
func (c *Config) Valid(ctx context.Context, token *oauth2.Token) bool {
	if len(c.whitelist) > 0 && !c.whitelist[c.Email(ctx, token)] {
		return false
	}
	if !token.Valid() {
		return false
	}
	svc, err := googleapi.NewService(ctx, option.WithHTTPClient(c.newClient(ctx, token)))
	if err != nil {
		return false
	}
	t := svc.Tokeninfo()
	t.AccessToken(token.AccessToken)
	ti, err := t.Do()
	if err != nil {
		return false
	}
	if ti.Audience != c.config.ClientID {
		return false
	}
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return false
	}
	if c.domain != "" && ui.Hd != c.domain {
		return false
	}
	metrics.M.AuthValid.WithLabelValues("google").Inc()
	return true
}

// Revoke disables the access token.
func (c *Config) Revoke(ctx context.Context, token *oauth2.Token) error {
	client := c.newClient(ctx, token)
	u := fmt.Sprintf(revokeURL, token.AccessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StartSession retrieves an authentication endpoint from Google.
func (c *Config) StartSession(state string) string {
	return c.config.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", c.domain))
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(ctx, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("google").Inc()
	}
	return t, err
}

// Email retrieves the email address of the user.
func (c *Config) Email(ctx context.Context, token *oauth2.Token) string {
	svc, err := googleapi.NewService(ctx, option.WithHTTPClient(c.newClient(ctx, token)))
	if err != nil {
		return ""
	}
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return ""
	}
	return ui.Email
}

// Username retrieves the username portion of the user's email address.
func (c *Config) Username(ctx context.Context, token *oauth2.Token) string {
	return strings.Split(c.Email(ctx, token), "@")[0]
}
