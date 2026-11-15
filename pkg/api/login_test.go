package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	claims "github.com/grafana/authlib/types"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/metrics/metricutil"
	"github.com/grafana/grafana/pkg/login/social"
	"github.com/grafana/grafana/pkg/models/usertoken"
	"github.com/grafana/grafana/pkg/services/auth/authtest"
	"github.com/grafana/grafana/pkg/services/authn"
	"github.com/grafana/grafana/pkg/services/authn/authntest"
	contextmodel "github.com/grafana/grafana/pkg/services/contexthandler/model"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/hooks"
	"github.com/grafana/grafana/pkg/services/licensing"
	"github.com/grafana/grafana/pkg/services/licensing/licensingtest"
	loginservice "github.com/grafana/grafana/pkg/services/login"
	"github.com/grafana/grafana/pkg/services/login/authinfotest"
	"github.com/grafana/grafana/pkg/services/navtree"
	"github.com/grafana/grafana/pkg/services/secrets"
	"github.com/grafana/grafana/pkg/services/secrets/fakes"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/setting"
)

const loginCookieName = "grafana_session"

// setupHTMLHandlerMetrics creates and registers the prometheus metrics needed for HTTPServer tests
// that call methods using htmlHandlerRequestsDuration (LoginView, Index, NotFoundHandler)
func setupHTMLHandlerMetrics() (prometheus.Registerer, *prometheus.HistogramVec) {
	promRegister := prometheus.NewRegistry()
	htmlHandlerRequestsDuration := metricutil.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "grafana",
		Name:      "html_handler_requests_duration_seconds",
	}, []string{"handler"})
	promRegister.MustRegister(htmlHandlerRequestsDuration)
	return promRegister, htmlHandlerRequestsDuration
}

func fakeSetIndexViewData(t *testing.T) {
	origSetIndexViewData := setIndexViewData
	t.Cleanup(func() {
		setIndexViewData = origSetIndexViewData
	})
	setIndexViewData = func(*HTTPServer, *contextmodel.ReqContext) (*dtos.IndexViewData, error) {
		data := &dtos.IndexViewData{
			User:     &dtos.CurrentUser{},
			Settings: &dtos.FrontendSettingsDTO{},
			NavTree:  &navtree.NavTreeRoot{},
			Assets: &dtos.EntryPointAssets{
				JSFiles: []dtos.EntryPointAsset{},
				Dark:    "dark.css",
				Light:   "light.css",
			},
		}
		return data, nil
	}
}

func fakeViewIndex(t *testing.T) {
	origGetViewIndex := getViewIndex
	t.Cleanup(func() {
		getViewIndex = origGetViewIndex
	})
	getViewIndex = func() string {
		return "index"
	}
}

func getBody(resp *httptest.ResponseRecorder) (string, error) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(responseData), nil
}

var oAuthInfos = map[string]*social.OAuthInfo{
	"github": {
		ClientId:     "fake",
		ClientSecret: "fakefake",
		Enabled:      true,
		AllowSignup:  true,
		Name:         "github",
	},
}

func TestLoginErrorCookieAPIEndpoint(t *testing.T) {
	fakeSetIndexViewData(t)

	fakeViewIndex(t)

	sc := setupScenarioContext(t, "/login")
	cfg := setting.NewCfg()
	secretsService := secretsManager.SetupTestService(t, fakes.NewFakeSecretsStore())
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		Cfg:                         cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: cfg},
		License:                     &licensing.OSSLicensingService{},
		SocialService:               &mockSocialService{},
		SecretsService:              secretsService,
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	cfg.LoginCookieName = loginCookieName
	cfg.OAuthAutoLogin = true

	oauthError := errors.New("User not a member of one of the required organizations")
	encryptedError, err := hs.SecretsService.Encrypt(context.Background(), []byte(oauthError.Error()), secrets.WithoutScope())
	require.NoError(t, err)
	expCookiePath := "/"
	if len(cfg.AppSubURL) > 0 {
		expCookiePath = cfg.AppSubURL
	}
	cookie := http.Cookie{
		Name:     loginErrorCookieName,
		MaxAge:   60,
		Value:    hex.EncodeToString(encryptedError),
		HttpOnly: true,
		Path:     expCookiePath,
		Secure:   hs.Cfg.CookieSecure,
		SameSite: hs.Cfg.CookieSameSiteMode,
	}
	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertionsWithCookie("GET", sc.url, cookie).exec()
	require.Equal(t, 200, sc.resp.Code)

	responseString, err := getBody(sc.resp)
	require.NoError(t, err)
	assert.True(t, strings.Contains(responseString, oauthError.Error()))
}

func TestLoginViewRedirect(t *testing.T) {
	fakeSetIndexViewData(t)
	fakeViewIndex(t)
	sc := setupScenarioContext(t, "/login")
	cfg := setting.NewCfg()
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		Cfg:                         cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: cfg},
		License:                     &licensing.OSSLicensingService{},
		SocialService:               &mockSocialService{},
		Features:                    featuremgmt.WithFeatures(),
		log:                         log.NewNopLogger(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}
	hs.Cfg.CookieSecure = true

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.IsSignedIn = true
		c.SignedInUser = &user.SignedInUser{
			UserID: 10,
		}
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	redirectCases := []struct {
		desc      string
		appSubURL string
		want      string
	}{
		{desc: "app root", want: "/"},
		{desc: "app root with subpath", appSubURL: "/grafana", want: "/grafana/"},
	}

	for _, c := range redirectCases {
		hs.Cfg.AppSubURL = c.appSubURL
		t.Run(c.desc, func(t *testing.T) {
			sc.m.Get(sc.url, sc.defaultHandler)
			sc.fakeReqNoAssertions("GET", sc.url)
			sc.req.AddCookie(&http.Cookie{Name: "redirect_to", Value: "/profile"})
			sc.exec()
			require.Equal(t, 302, sc.resp.Code)
			location, ok := sc.resp.Header()["Location"]
			assert.True(t, ok)
			assert.Equal(t, c.want, location[0])
			for _, cookieValue := range sc.resp.Header()["Set-Cookie"] {
				assert.NotContains(t, cookieValue, "redirect_to=")
			}
		})
	}
}

func TestLoginPostRedirect(t *testing.T) {
	fakeSetIndexViewData(t)

	fakeViewIndex(t)
	sc := setupScenarioContext(t, "/login")

	hs := &HTTPServer{
		log:          log.NewNopLogger(),
		Cfg:          setting.NewCfg(),
		HooksService: &hooks.HooksService{},
		License:      &licensing.OSSLicensingService{},
		authnService: &authntest.FakeService{
			ExpectedIdentity: &authn.Identity{ID: "42", Type: claims.TypeUser, SessionToken: &usertoken.UserToken{}},
		},
		AuthTokenService: authtest.NewFakeUserAuthTokenService(),
		Features:         featuremgmt.WithFeatures(),
	}
	hs.Cfg.CookieSecure = true

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.Req.Header.Set("Content-Type", "application/json")
		c.Req.Body = io.NopCloser(bytes.NewBufferString(`{"user":"admin","password":"admin"}`))
		return hs.LoginPost(c)
	})

	redirectCases := []struct {
		desc      string
		appSubURL string
		want      string
	}{
		{desc: "app root", want: "/"},
		{desc: "app root with subpath", appSubURL: "/grafana", want: "/grafana/"},
	}

	for _, c := range redirectCases {
		hs.Cfg.AppSubURL = c.appSubURL

		t.Run(c.desc, func(t *testing.T) {
			sc.m.Post(sc.url, sc.defaultHandler)
			sc.fakeReqNoAssertions("POST", sc.url)
			sc.req.AddCookie(&http.Cookie{Name: "redirect_to", Value: "/profile"})
			sc.exec()
			require.Equal(t, 200, sc.resp.Code)

			respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
			require.NoError(t, err)
			assert.Equal(t, c.want, respJSON.Get("redirectUrl").MustString())
			for _, cookieValue := range sc.resp.Header()["Set-Cookie"] {
				assert.NotContains(t, cookieValue, "redirect_to=")
			}
		})
	}
}

func TestLoginOAuthRedirect(t *testing.T) {
	fakeSetIndexViewData(t)

	sc := setupScenarioContext(t, "/login")
	cfg := setting.NewCfg()
	mock := &mockSocialService{
		oAuthInfo: &social.OAuthInfo{
			ClientId:     "fake",
			ClientSecret: "fakefake",
			Enabled:      true,
			AllowSignup:  true,
			Name:         "github",
		},
		oAuthInfos: oAuthInfos,
	}
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		authnService:                &authntest.FakeService{},
		Cfg:                         cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: cfg},
		License:                     &licensing.OSSLicensingService{},
		SocialService:               mock,
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	hs.Cfg.OAuthAutoLogin = true
	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertions("GET", sc.url).exec()

	require.Equal(t, 307, sc.resp.Code)
	location, ok := sc.resp.Header()["Location"]
	assert.True(t, ok)
	assert.Equal(t, "/login/github?redirectTo=", location[0])
}

func TestLoginOAuthRedirectKeepsRedirectTo(t *testing.T) {
	fakeSetIndexViewData(t)

	sc := setupScenarioContext(t, "/login")
	cfg := setting.NewCfg()
	mock := &mockSocialService{
		oAuthInfo: &social.OAuthInfo{
			ClientId:     "fake",
			ClientSecret: "fakefake",
			Enabled:      true,
			AllowSignup:  true,
			Name:         "github",
		},
		oAuthInfos: oAuthInfos,
	}
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		authnService:                &authntest.FakeService{},
		Cfg:                         cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: cfg},
		License:                     &licensing.OSSLicensingService{},
		SocialService:               mock,
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	hs.Cfg.OAuthAutoLogin = true
	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqWithParams("GET", sc.url, map[string]string{"redirectTo": "/d/abc"}).exec()

	require.Equal(t, 307, sc.resp.Code)
	assert.Equal(t, "/login/github?redirectTo=%2Fd%2Fabc", sc.resp.Header().Get("Location"))
}

func TestLoginInternal(t *testing.T) {
	fakeSetIndexViewData(t)

	fakeViewIndex(t)
	sc := setupScenarioContext(t, "/login")
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		Cfg:                         setting.NewCfg(),
		License:                     &licensing.OSSLicensingService{},
		log:                         log.New("test"),
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.Req.URL.RawQuery = "disableAutoLogin=true"
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	hs.Cfg.OAuthAutoLogin = true
	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertions("GET", sc.url).exec()

	// Shouldn't redirect to the OAuth login URL
	assert.Equal(t, 200, sc.resp.Code)
}

func TestAuthProxyLoginEnableLoginTokenDisabled(t *testing.T) {
	sc := setupAuthProxyLoginTest(t, false)

	require.Equal(t, 302, sc.resp.Code)
	location, ok := sc.resp.Header()["Location"]
	assert.True(t, ok)
	assert.Equal(t, "/", location[0])

	_, ok = sc.resp.Header()["Set-Cookie"]
	assert.False(t, ok, "Set-Cookie does not exist")
}

func TestAuthProxyLoginWithEnableLoginToken(t *testing.T) {
	sc := setupAuthProxyLoginTest(t, true)
	require.Equal(t, 302, sc.resp.Code)

	location, ok := sc.resp.Header()["Location"]
	assert.True(t, ok)
	assert.Equal(t, "/", location[0])
	setCookie := sc.resp.Header()["Set-Cookie"]
	require.NotNil(t, setCookie, "Set-Cookie should exist")
	assert.Equal(t, fmt.Sprintf("%s=; Path=/; Max-Age=0; HttpOnly", loginCookieName), setCookie[0])
}

func TestAuthProxyLoginWithEnableLoginTokenAndEnabledOauthAutoLogin(t *testing.T) {
	fakeSetIndexViewData(t)

	mock := &mockSocialService{
		oAuthInfo: &social.OAuthInfo{
			ClientId:     "fake",
			ClientSecret: "fakefake",
			Enabled:      true,
			AllowSignup:  true,
			Name:         "github",
		},
		oAuthInfos: oAuthInfos,
	}

	sc := setupScenarioContext(t, "/login")
	sc.cfg.LoginCookieName = loginCookieName
	sc.cfg.OAuthAutoLogin = true
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		Cfg:                         sc.cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: sc.cfg},
		License:                     &licensing.OSSLicensingService{},
		AuthTokenService:            authtest.NewFakeUserAuthTokenService(),
		log:                         log.New("hello"),
		SocialService:               mock,
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.IsSignedIn = true
		c.SignedInUser = &user.SignedInUser{
			UserID:          10,
			AuthenticatedBy: loginservice.AuthProxyAuthModule,
		}
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	sc.cfg.AuthProxy.Enabled = true
	sc.cfg.AuthProxy.EnableLoginToken = true

	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertions("GET", sc.url).exec()
	require.Equal(t, 302, sc.resp.Code)

	location, ok := sc.resp.Header()["Location"]
	assert.True(t, ok)
	assert.Equal(t, "/", location[0])
	setCookie := sc.resp.Header()["Set-Cookie"]
	require.NotNil(t, setCookie, "Set-Cookie should exist")
	assert.Equal(t, fmt.Sprintf("%s=; Path=/; Max-Age=0; HttpOnly", loginCookieName), setCookie[0])
}

func setupAuthProxyLoginTest(t *testing.T, enableLoginToken bool) *scenarioContext {
	fakeSetIndexViewData(t)

	sc := setupScenarioContext(t, "/login")
	sc.cfg.LoginCookieName = loginCookieName
	promRegister, htmlHandlerRequestsDuration := setupHTMLHandlerMetrics()
	hs := &HTTPServer{
		Cfg:                         sc.cfg,
		SettingsProvider:            &setting.OSSImpl{Cfg: sc.cfg},
		License:                     &licensing.OSSLicensingService{},
		AuthTokenService:            authtest.NewFakeUserAuthTokenService(),
		log:                         log.New("hello"),
		SocialService:               &mockSocialService{},
		Features:                    featuremgmt.WithFeatures(),
		promRegister:                promRegister,
		htmlHandlerRequestsDuration: htmlHandlerRequestsDuration,
	}

	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.IsSignedIn = true
		c.SignedInUser = &user.SignedInUser{
			UserID:          10,
			AuthenticatedBy: loginservice.AuthProxyAuthModule,
		}
		hs.LoginView(c)
		return response.Empty(http.StatusOK)
	})

	sc.cfg.AuthProxy.Enabled = true
	sc.cfg.AuthProxy.EnableLoginToken = enableLoginToken

	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertions("GET", sc.url).exec()

	return sc
}

func TestLogoutSaml(t *testing.T) {
	fakeSetIndexViewData(t)
	fakeViewIndex(t)
	sc := setupScenarioContextSamlLogout(t, "/logout")
	license := licensingtest.NewFakeLicensing()
	license.On("FeatureEnabled", "saml").Return(true)

	hs := &HTTPServer{
		authnService: &authntest.FakeService{
			ExpectedClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSingleLogoutEnabled: true,
			},
		},
		Cfg:              sc.cfg,
		SettingsProvider: &setting.OSSImpl{Cfg: sc.cfg},
		License:          license,
		SocialService:    &mockSocialService{},
		Features:         featuremgmt.WithFeatures(),
		authInfoService: &authinfotest.FakeService{
			ExpectedUserAuth: &loginservice.UserAuth{AuthModule: loginservice.SAMLAuthModule},
		},
	}

	assert.Equal(t, true, hs.samlSingleLogoutEnabled())
	sc.defaultHandler = routing.Wrap(func(c *contextmodel.ReqContext) response.Response {
		c.SignedInUser = &user.SignedInUser{
			UserID:          1,
			AuthenticatedBy: loginservice.SAMLAuthModule,
		}
		hs.Logout(c)
		return response.Empty(http.StatusOK)
	})
	sc.m.Get(sc.url, sc.defaultHandler)
	sc.fakeReqNoAssertions("GET", sc.url).exec()
	require.Equal(t, 302, sc.resp.Code)
}

func TestIsExternallySynced(t *testing.T) {
	testcases := []struct {
		name                string
		cfg                 *setting.Cfg
		provider            string
		enabledAuthnClients []string
		authnClientConfig   authn.SSOClientConfig
		expected            bool
	}{
		// Same for all of the OAuth providers
		{
			name:                "AzureAD external user should return that it is externally synced",
			cfg:                 &setting.Cfg{},
			provider:            loginservice.AzureADAuthModule,
			enabledAuthnClients: []string{authn.ClientWithPrefix("azuread")},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: false,
			},
			expected: true,
		},
		{
			name:                "AzureAD external user should return that it is not externally synced when org role sync is set",
			cfg:                 &setting.Cfg{},
			provider:            loginservice.AzureADAuthModule,
			enabledAuthnClients: []string{authn.ClientWithPrefix(strings.TrimPrefix(loginservice.AzureADAuthModule, "oauth_"))},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: true,
			},
			expected: false,
		},
		{
			name:                "AzureAD external user should return that it is not externally synced when the provider is not enabled",
			cfg:                 &setting.Cfg{},
			enabledAuthnClients: []string{},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: false,
			},
			provider: loginservice.AzureADAuthModule,
			expected: false,
		},
		// saml
		{
			name:                "SAML synced user should return that it is not externally synced when the provider is disabled",
			cfg:                 &setting.Cfg{},
			provider:            loginservice.SAMLAuthModule,
			enabledAuthnClients: []string{},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: false,
			},
			expected: false,
		},
		{
			name:                "SAML synced user should return that it is externally synced",
			cfg:                 &setting.Cfg{},
			provider:            loginservice.SAMLAuthModule,
			enabledAuthnClients: []string{authn.ClientSAML},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: false,
			},
			expected: true,
		},
		{
			name:                "SAML synced user should return that it is not externally synced when org role sync is set",
			cfg:                 &setting.Cfg{},
			provider:            loginservice.SAMLAuthModule,
			enabledAuthnClients: []string{authn.ClientSAML},
			authnClientConfig: &authntest.FakeSSOClientConfig{
				ExpectedIsSkipOrgRoleSyncEnabled: true,
			},
			expected: false,
		},
		// ldap
		{
			name:     "LDAP synced user should return that it is externally synced",
			cfg:      &setting.Cfg{LDAPAuthEnabled: true, LDAPSkipOrgRoleSync: false},
			provider: loginservice.LDAPAuthModule,
			expected: true,
		},
		{
			name:     "LDAP synced user should return that it is not externally synced when org role sync is set",
			cfg:      &setting.Cfg{LDAPAuthEnabled: true, LDAPSkipOrgRoleSync: true},
			provider: loginservice.LDAPAuthModule,
			expected: false,
		},
		// jwt
		{
			name:     "JWT synced user should return that it is externally synced",
			cfg:      &setting.Cfg{JWTAuth: setting.AuthJWTSettings{Enabled: true, SkipOrgRoleSync: false}},
			provider: loginservice.JWTModule,
			expected: true,
		},
		{
			name:     "JWT synced user should return that it is not externally synced when org role sync is set",
			cfg:      &setting.Cfg{JWTAuth: setting.AuthJWTSettings{Enabled: true, SkipOrgRoleSync: true}},
			provider: loginservice.JWTModule,
			expected: false,
		},
		// IsProvider test
		{
			name:     "If no provider enabled should return false",
			cfg:      &setting.Cfg{JWTAuth: setting.AuthJWTSettings{Enabled: false, SkipOrgRoleSync: true}},
			provider: loginservice.JWTModule,
			expected: false,
		},
	}

	for _, tc := range testcases {
		hs := &HTTPServer{
			authnService: &authntest.FakeService{
				ExpectedClientConfig: tc.authnClientConfig,
				EnabledClients:       tc.enabledAuthnClients,
			},
		}

		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, hs.isExternallySynced(tc.cfg, tc.provider))
		})
	}
}

func TestIsProviderEnabled(t *testing.T) {
	testcases := []struct {
		name                string
		provider            string
		enabledAuthnClients []string
		expected            bool
	}{
		{
			name:                "Github should return true if enabled",
			provider:            loginservice.GithubAuthModule,
			enabledAuthnClients: []string{authn.ClientWithPrefix(strings.TrimPrefix(loginservice.GithubAuthModule, "oauth_"))},
			expected:            true,
		},
		{
			name:                "Github should return false if not enabled",
			provider:            loginservice.GithubAuthModule,
			enabledAuthnClients: []string{},
			expected:            false,
		},
		// saml
		{
			name:                "SAML should return true if enabled",
			provider:            loginservice.SAMLAuthModule,
			enabledAuthnClients: []string{authn.ClientSAML},
			expected:            true,
		},
		{
			name:                "SAML should return false if not enabled",
			provider:            loginservice.SAMLAuthModule,
			enabledAuthnClients: []string{},
			expected:            false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			hs := &HTTPServer{
				authnService: &authntest.FakeService{
					EnabledClients: tc.enabledAuthnClients,
				},
			}
			assert.Equal(t, tc.expected, hs.isProviderEnabled(setting.NewCfg(), tc.provider))
		})
	}
}

type mockSocialService struct {
	oAuthInfo       *social.OAuthInfo
	oAuthInfos      map[string]*social.OAuthInfo
	oAuthProviders  map[string]bool
	httpClient      *http.Client
	socialConnector social.SocialConnector
	err             error
}

func (m *mockSocialService) GetOAuthInfoProvider(name string) *social.OAuthInfo {
	return m.oAuthInfo
}

func (m *mockSocialService) GetOAuthInfoProviders() map[string]*social.OAuthInfo {
	return m.oAuthInfos
}

func (m *mockSocialService) GetOAuthProviders() map[string]bool {
	return m.oAuthProviders
}

func (m *mockSocialService) GetOAuthHttpClient(name string) (*http.Client, error) {
	return m.httpClient, m.err
}

func (m *mockSocialService) GetConnector(string) (social.SocialConnector, error) {
	return m.socialConnector, m.err
}
