/*
Copyright 2015 All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

// newDefaultConfig returns a initialized config
func newDefaultConfig() *Config {
	return &Config{
		Listen:                   "127.0.0.1:3000",
		TagData:                  make(map[string]string, 0),
		MatchClaims:              make(map[string]string, 0),
		Headers:                  make(map[string]string, 0),
		UpstreamTimeout:          time.Duration(10) * time.Second,
		UpstreamKeepaliveTimeout: time.Duration(10) * time.Second,
		CookieAccessName:         "kc-access",
		CookieRefreshName:        "kc-state",
		SecureCookie:             true,
		SkipUpstreamTLSVerify:    true,
		CrossOrigin:              CORS{},
	}
}

// isValid validates if the config is valid
func (r *Config) isValid() error {
	if r.Listen == "" {
		return fmt.Errorf("you have not specified the listening interface")
	}
	if r.TLSCertificate != "" && r.TLSPrivateKey == "" {
		return fmt.Errorf("you have not provided a private key")
	}
	if r.TLSPrivateKey != "" && r.TLSCertificate == "" {
		return fmt.Errorf("you have not provided a certificate file")
	}
	if r.TLSCertificate != "" && !fileExists(r.TLSCertificate) {
		return fmt.Errorf("the tls certificate %s does not exist", r.TLSCertificate)
	}
	if r.TLSPrivateKey != "" && !fileExists(r.TLSPrivateKey) {
		return fmt.Errorf("the tls private key %s does not exist", r.TLSPrivateKey)
	}
	if r.TLSCaCertificate != "" && !fileExists(r.TLSCaCertificate) {
		return fmt.Errorf("the tls ca certificate file %s does not exist", r.TLSCaCertificate)
	}
	if r.TLSClientCertificate != "" && !fileExists(r.TLSClientCertificate) {
		return fmt.Errorf("the tls client certificate %s does not exist", r.TLSClientCertificate)
	}

	if r.EnableForwarding {
		if r.ClientID == "" {
			return fmt.Errorf("you have not specified the client id")
		}
		if r.DiscoveryURL == "" {
			return fmt.Errorf("you have not specified the discovery url")
		}
		if r.ForwardingUsername == "" {
			return fmt.Errorf("no forwarding username")
		}
		if r.ForwardingPassword == "" {
			return fmt.Errorf("no forwarding password")
		}
	} else {
		if r.Upstream == "" {
			return fmt.Errorf("you have not specified an upstream endpoint to proxy to")
		}
		if _, err := url.Parse(r.Upstream); err != nil {
			return fmt.Errorf("the upstream endpoint is invalid, %s", err)
		}
		// step: if the skip verification is off, we need the below
		if !r.SkipTokenVerification {
			if r.ClientID == "" {
				return fmt.Errorf("you have not specified the client id")
			}
			if r.DiscoveryURL == "" {
				return fmt.Errorf("you have not specified the discovery url")
			}
			if strings.HasSuffix(r.RedirectionURL, "/") {
				r.RedirectionURL = strings.TrimSuffix(r.RedirectionURL, "/")
			}
			if r.EnableRefreshTokens && r.EncryptionKey == "" {
				return fmt.Errorf("you have not specified a encryption key for encoding the session state")
			}
			if r.EnableRefreshTokens && (len(r.EncryptionKey) != 16 && len(r.EncryptionKey) != 32) {
				return fmt.Errorf("the encryption key (%d) must be either 16 or 32 characters for AES-128/AES-256 selection", len(r.EncryptionKey))
			}
			if !r.NoRedirects && r.SecureCookie && !strings.HasPrefix(r.RedirectionURL, "https") {
				return fmt.Errorf("the cookie is set to secure but your redirection url is non-tls")
			}
			if r.StoreURL != "" {
				if _, err := url.Parse(r.StoreURL); err != nil {
					return fmt.Errorf("the store url is invalid, error: %s", err)
				}
			}
		}
		// step: valid the resources
		for _, resource := range r.Resources {
			if err := resource.IsValid(); err != nil {
				return err
			}
		}
		// step: validate the claims are validate regex's
		for k, claim := range r.MatchClaims {
			// step: validate the regex
			if _, err := regexp.Compile(claim); err != nil {
				return fmt.Errorf("the claim matcher: %s for claim: %s is not a valid regex", claim, k)
			}
		}
	}

	return nil
}

// hasCustomSignInPage checks if there is a custom sign in  page
func (r *Config) hasCustomSignInPage() bool {
	if r.SignInPage != "" {
		return true
	}

	return false
}

// hasForbiddenPage checks if there is a custom forbidden page
func (r *Config) hasCustomForbiddenPage() bool {
	if r.ForbiddenPage != "" {
		return true
	}

	return false
}

//
// readOptions parses the command line options and constructs a config object
// @TODO look for a shorter way of doing this, we're maintaining the same options in multiple places, it's tedious!
//
func readOptions(cx *cli.Context, config *Config) (err error) {
	if cx.String("listen") != "" {
		config.Listen = cx.String("listen")
	}
	if cx.String("client-secret") != "" {
		config.ClientSecret = cx.String("client-secret")
	}
	if cx.String("client-id") != "" {
		config.ClientID = cx.String("client-id")
	}
	if cx.String("discovery-url") != "" {
		config.DiscoveryURL = cx.String("discovery-url")
	}
	if cx.String("upstream-url") != "" {
		config.Upstream = cx.String("upstream-url")
	}
	if cx.String("revocation-url") != "" {
		config.RevocationEndpoint = cx.String("revocation-url")
	}
	if cx.IsSet("upstream-keepalives") {
		config.UpstreamKeepalives = cx.Bool("upstream-keepalives")
	}
	if cx.IsSet("upstream-timeout") {
		config.UpstreamTimeout = cx.Duration("upstream-timeout")
	}
	if cx.IsSet("upstream-keepalive-timeout") {
		config.UpstreamKeepaliveTimeout = cx.Duration("upstream-keepalive-timeout")
	}
	if cx.IsSet("idle-duration") {
		config.IdleDuration = cx.Duration("idle-duration")
	}
	if cx.IsSet("skip-token-verification") {
		config.SkipTokenVerification = cx.Bool("skip-token-verification")
	}
	if cx.IsSet("skip-upstream-tls-verify") {
		config.SkipUpstreamTLSVerify = cx.Bool("skip-upstream-tls-verify")
	}
	if cx.IsSet("encryption-key") {
		config.EncryptionKey = cx.String("encryption-key")
	}
	if cx.IsSet("secure-cookie") {
		config.SecureCookie = cx.Bool("secure-cookie")
	}
	if cx.IsSet("cookie-access-name") {
		config.CookieAccessName = cx.String("cookie-access-name")
	}
	if cx.IsSet("cookie-refresh-name") {
		config.CookieRefreshName = cx.String("cookie-refresh-name")
	}
	if cx.IsSet("cookie-domain") {
		config.CookieDomain = cx.String("cookie-domain")
	}
	if cx.IsSet("add-claims") {
		config.AddClaims = append(config.AddClaims, cx.StringSlice("add-claims")...)
	}
	if cx.String("store-url") != "" {
		config.StoreURL = cx.String("store-url")
	}
	if cx.IsSet("no-redirects") {
		config.NoRedirects = cx.Bool("no-redirects")
	}
	if cx.String("redirection-url") != "" {
		config.RedirectionURL = cx.String("redirection-url")
	}
	if cx.IsSet("tls-cert") {
		config.TLSCertificate = cx.String("tls-cert")
	}
	if cx.IsSet("tls-private-key") {
		config.TLSPrivateKey = cx.String("tls-private-key")
	}
	if cx.IsSet("tls-ca-certificate") {
		config.TLSCaCertificate = cx.String("tls-ca-certificate")
	}
	if cx.IsSet("tls-client-certificate") {
		config.TLSClientCertificate = cx.String("tls-client-certificate")
	}
	if cx.IsSet("enable-metrics") {
		config.EnableMetrics = cx.Bool("enable-metrics")
	}
	if cx.IsSet("enable-proxy-protocol") {
		config.EnableProxyProtocol = cx.Bool("enable-proxy-protocol")
	}
	if cx.IsSet("enable-forwarding") {
		config.EnableForwarding = cx.Bool("enable-forwarding")
	}
	if cx.IsSet("enable-refresh-tokens") {
		config.EnableRefreshTokens = cx.Bool("enable-refresh-tokens")
	}
	if cx.IsSet("forwarding-username") {
		config.ForwardingUsername = cx.String("forwarding-username")
	}
	if cx.IsSet("forwarding-password") {
		config.ForwardingPassword = cx.String("forwarding-password")
	}
	if cx.IsSet("forwarding-domains") {
		config.ForwardingDomains = append(config.ForwardingDomains, cx.StringSlice("forwarding-domains")...)
	}
	if cx.IsSet("signin-page") {
		config.SignInPage = cx.String("signin-page")
	}
	if cx.IsSet("forbidden-page") {
		config.ForbiddenPage = cx.String("forbidden-page")
	}
	if cx.IsSet("enable-security-filter") {
		config.EnableSecurityFilter = true
	}
	if cx.IsSet("json-logging") {
		config.LogJSONFormat = cx.Bool("json-logging")
	}
	if cx.IsSet("log-requests") {
		config.LogRequests = cx.Bool("log-requests")
	}
	if cx.IsSet("verbose") {
		config.Verbose = cx.Bool("verbose")
	}
	if cx.IsSet("scope") {
		config.Scopes = cx.StringSlice("scope")
	}
	if cx.IsSet("hostname") {
		config.Hostnames = append(config.Hostnames, cx.StringSlice("hostname")...)
	}
	if cx.IsSet("cors-origins") {
		config.CrossOrigin.Origins = append(config.CrossOrigin.Origins, cx.StringSlice("cors-origins")...)
	}
	if cx.IsSet("cors-methods") {
		config.CrossOrigin.Methods = append(config.CrossOrigin.Methods, cx.StringSlice("cors-methods")...)
	}
	if cx.IsSet("cors-headers") {
		config.CrossOrigin.Headers = append(config.CrossOrigin.Headers, cx.StringSlice("cors-headers")...)
	}
	if cx.IsSet("cors-exposed-headers") {
		config.CrossOrigin.ExposedHeaders = append(config.CrossOrigin.ExposedHeaders, cx.StringSlice("cors-exposed-headers")...)
	}
	if cx.IsSet("cors-max-age") {
		config.CrossOrigin.MaxAge = cx.Duration("cors-max-age")
	}
	if cx.IsSet("cors-credentials") {
		config.CrossOrigin.Credentials = cx.BoolT("cors-credentials")
	}
	if cx.IsSet("tag") {
		tags, err := decodeKeyPairs(cx.StringSlice("tag"))
		if err != nil {
			return err
		}
		mergeMaps(config.MatchClaims, tags)
	}
	if cx.IsSet("match-claims") {
		claims, err := decodeKeyPairs(cx.StringSlice("match-claims"))
		if err != nil {
			return err
		}
		mergeMaps(config.MatchClaims, claims)
	}
	if cx.IsSet("headers") {
		headers, err := decodeKeyPairs(cx.StringSlice("headers"))
		if err != nil {
			return err
		}
		mergeMaps(config.MatchClaims, headers)
	}
	if cx.IsSet("resource") {
		for _, x := range cx.StringSlice("resource") {
			resource, err := newResource().Parse(x)
			if err != nil {
				return fmt.Errorf("invalid resource %s, %s", x, err)
			}
			config.Resources = append(config.Resources, resource)
		}
	}

	return nil
}

// readConfigFile reads and parses the configuration file
func readConfigFile(filename string, config *Config) error {
	// step: read in the contents of the file
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	// step: attempt to un-marshal the data
	switch ext := filepath.Ext(filename); ext {
	case "json":
		err = json.Unmarshal(content, config)
	default:
		err = yaml.Unmarshal(content, config)
	}

	return err
}

// getOptions returns the command line options
func getOptions() []cli.Flag {
	defaults := newDefaultConfig()

	return []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "the path to the configuration file for the keycloak proxy",
			EnvVar: "PROXY_CONFIG_FILE",
		},
		cli.StringFlag{
			Name:   "listen",
			Usage:  "the interface the service should be listening on",
			Value:  defaults.Listen,
			EnvVar: "PROXY_LISTEN",
		},
		cli.StringFlag{
			Name:   "client-secret",
			Usage:  "the client secret used to authenticate to the oauth server (access_type: confidential)",
			EnvVar: "PROXY_CLIENT_SECRET",
		},
		cli.StringFlag{
			Name:   "client-id",
			Usage:  "the client id used to authenticate to the oauth service",
			EnvVar: "PROXY_CLIENT_ID",
		},
		cli.StringFlag{
			Name:   "discovery-url",
			Usage:  "the discovery url to retrieve the openid configuration",
			EnvVar: "PROXY_DISCOVERY_URL",
		},
		cli.StringSliceFlag{
			Name:  "scope",
			Usage: "a variable list of scopes requested when authenticating the user",
		},
		cli.BoolFlag{
			Name:  "token-validate-only",
			Usage: "validate the token and roles only, no required implement oauth",
		},
		cli.DurationFlag{
			Name:  "idle-duration",
			Usage: "the expiration of the access token cookie, if not used within this time its removed",
		},
		cli.StringFlag{
			Name:   "redirection-url",
			Usage:  fmt.Sprintf("redirection url for the oauth callback url (%s is added)", oauthURL),
			EnvVar: "PROXY_REDIRECTION_URL",
		},
		cli.StringFlag{
			Name:   "revocation-url",
			Usage:  "the url for the revocation endpoint to revoke refresh token",
			Value:  "/oauth2/revoke",
			EnvVar: "PROXY_REVOCATION_URL",
		},
		cli.StringFlag{
			Name:   "store-url",
			Usage:  "url for the storage subsystem, e.g redis://127.0.0.1:6379, file:///etc/tokens.file",
			EnvVar: "PROXY_STORE_URL",
		},
		cli.StringFlag{
			Name:   "upstream-url",
			Usage:  "the url for the upstream endpoint you wish to proxy to",
			Value:  defaults.Upstream,
			EnvVar: "PROXY_UPSTREAM_URL",
		},
		cli.BoolTFlag{
			Name:  "upstream-keepalives",
			Usage: "enables or disables the keepalive connections for upstream endpoint",
		},
		cli.DurationFlag{
			Name:  "upstream-timeout",
			Usage: "is the maximum amount of time a dial will wait for a connect to complete",
			Value: defaults.UpstreamTimeout,
		},
		cli.DurationFlag{
			Name:  "upstream-keepalive-timeout",
			Usage: "specifies the keep-alive period for an active network connection",
			Value: defaults.UpstreamKeepaliveTimeout,
		},
		cli.BoolFlag{
			Name:  "enable-refresh-tokens",
			Usage: "enables the handling of the refresh tokens",
		},
		cli.BoolTFlag{
			Name:  "secure-cookie",
			Usage: "enforces the cookie to be secure, default to true",
		},
		cli.StringSliceFlag{
			Name:  "cookie-domain",
			Usage: "a domain the access cookie is available to, defaults host header",
		},
		cli.StringFlag{
			Name:  "cookie-access-name",
			Usage: "the name of the cookie use to hold the access token",
			Value: defaults.CookieAccessName,
		},
		cli.StringFlag{
			Name:  "cookie-refresh-name",
			Usage: "the name of the cookie used to hold the encrypted refresh token",
			Value: defaults.CookieRefreshName,
		},
		cli.StringFlag{
			Name:  "encryption-key",
			Usage: "the encryption key used to encrpytion the session state",
		},
		cli.BoolFlag{
			Name:  "no-redirects",
			Usage: "do not have back redirects when no authentication is present, 401 them",
		},
		cli.StringSliceFlag{
			Name:  "hostname",
			Usage: "a list of hostnames the service will respond to, defaults to all",
		},
		cli.BoolFlag{
			Name:  "enable-metrics",
			Usage: "enable the prometheus metrics collector on /oauth/metrics",
		},
		cli.BoolFlag{
			Name:  "enable-proxy-protocol",
			Usage: "whether to enable proxy protocol",
		},
		cli.BoolFlag{
			Name:  "enable-forwarding",
			Usage: "enables the forwarding proxy mode, signing outbound request",
		},
		cli.StringFlag{
			Name:  "forwarding-username",
			Usage: "the username to use when logging into the openid provider",
		},
		cli.StringFlag{
			Name:  "forwarding-password",
			Usage: "the password to use when logging into the openid provider",
		},
		cli.StringSliceFlag{
			Name:  "forwarding-domains",
			Usage: "a list of domains which should be signed; everything else is relayed unsigned",
		},
		cli.StringFlag{
			Name:  "tls-cert",
			Usage: "the path to a certificate file used for TLS",
		},
		cli.StringFlag{
			Name:  "tls-private-key",
			Usage: "the path to the private key for TLS support",
		},
		cli.StringFlag{
			Name:  "tls-ca-certificate",
			Usage: "the path to the ca certificate used for mutual TLS",
		},
		cli.StringFlag{
			Name:  "tls-client-certificate",
			Usage: "the path to the client certificate, used to outbound connections in reverse and forwarding proxy modes",
		},
		cli.BoolTFlag{
			Name:  "skip-upstream-tls-verify",
			Usage: "whether to skip the verification of any upstream TLS (defaults to true)",
		},
		cli.StringSliceFlag{
			Name:  "match-claims",
			Usage: "keypair values for matching access token claims e.g. aud=myapp, iss=http://example.*",
		},
		cli.StringSliceFlag{
			Name:  "add-claims",
			Usage: "retrieve extra claims from the token and inject into headers, e.g given_name -> X-Auth-Given-Name",
		},
		cli.StringSliceFlag{
			Name:  "resource",
			Usage: "a list of resources 'uri=/admin|methods=GET|roles=role1,role2'",
		},
		cli.StringSliceFlag{
			Name:  "headers",
			Usage: "Add custom headers to the upstream request, key=value",
		},
		cli.StringFlag{
			Name:  "signin-page",
			Usage: "a custom template displayed for signin",
		},
		cli.StringFlag{
			Name:  "forbidden-page",
			Usage: "a custom template used for access forbidden",
		},
		cli.StringSliceFlag{
			Name:  "tag",
			Usage: "keypair's passed to the templates at render,e.g title='My Page'",
		},
		cli.StringSliceFlag{
			Name:  "cors-origins",
			Usage: "list of origins to add to the CORE origins control (Access-Control-Allow-Origin)",
		},
		cli.StringSliceFlag{
			Name:  "cors-methods",
			Usage: "the method permitted in the access control (Access-Control-Allow-Methods)",
		},
		cli.StringSliceFlag{
			Name:  "cors-headers",
			Usage: "a set of headers to add to the CORS access control (Access-Control-Allow-Headers)",
		},
		cli.StringSliceFlag{
			Name:  "cors-exposes-headers",
			Usage: "set the expose cors headers access control (Access-Control-Expose-Headers)",
		},
		cli.DurationFlag{
			Name:  "cors-max-age",
			Usage: "the max age applied to cors headers (Access-Control-Max-Age)",
		},
		cli.BoolFlag{
			Name:  "cors-credentials",
			Usage: "the credentials access control header (Access-Control-Allow-Credentials)",
		},
		cli.BoolFlag{
			Name:  "enable-security-filter",
			Usage: "enables the security filter handler",
		},
		cli.BoolFlag{
			Name:  "skip-token-verification",
			Usage: "TESTING ONLY; bypass token verification, only expiration and roles enforced",
		},
		cli.BoolTFlag{
			Name:  "json-logging",
			Usage: "switch on json logging rather than text (defaults true)",
		},
		cli.BoolTFlag{
			Name:  "log-requests",
			Usage: "switch on logging of all incoming requests (defaults true)",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "switch on debug / verbose logging",
		},
	}
}
