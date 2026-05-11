package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestApp_GetMetaEmbeddedSignupConfig_Disabled(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetMetaEmbeddedSignupConfig(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var data struct {
		Enabled           bool   `json:"enabled"`
		AppID             string `json:"app_id"`
		ConfigID          string `json:"config_id"`
		GraphAPIVersion   string `json:"graph_api_version"`
	}
	testutil.ParseEnvelopeResponse(t, req, &data)
	assert.False(t, data.Enabled)
	assert.Empty(t, data.AppID)
	assert.Empty(t, data.ConfigID)
	assert.Equal(t, "v18.0", data.GraphAPIVersion)
}

func TestApp_GetMetaEmbeddedSignupConfig_Enabled(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, withWhatsAppConfig(config.WhatsAppConfig{
		APIVersion:             "v25.0",
		BaseURL:                "https://graph.facebook.com",
		MetaAppID:              "123456789",
		MetaAppSecret:          "secret",
		EmbeddedSignupConfigID: "cfg-abc",
	}))

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.GetMetaEmbeddedSignupConfig(req)
	require.NoError(t, err)

	var data struct {
		Enabled         bool   `json:"enabled"`
		AppID           string `json:"app_id"`
		ConfigID        string `json:"config_id"`
		GraphAPIVersion string `json:"graph_api_version"`
	}
	testutil.ParseEnvelopeResponse(t, req, &data)
	assert.True(t, data.Enabled)
	assert.Equal(t, "123456789", data.AppID)
	assert.Equal(t, "cfg-abc", data.ConfigID)
	assert.Equal(t, "v25.0", data.GraphAPIVersion)
}

func TestApp_CreateAccountEmbeddedSignup_Success(t *testing.T) {
	t.Parallel()

	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v22.0/oauth/access_token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if r.FormValue("client_id") != "appid" || r.FormValue("client_secret") != "sec" || r.FormValue("code") != "authcode" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "EAAG_LONG_TOKEN"})
	}))
	defer oauthSrv.Close()

	app := newTestApp(t,
		withWhatsAppConfig(config.WhatsAppConfig{
			APIVersion:             "v22.0",
			BaseURL:                oauthSrv.URL,
			MetaAppID:              "appid",
			MetaAppSecret:          "sec",
			EmbeddedSignupConfigID: "cfg",
		}),
		withHTTPClient(oauthSrv.Client()),
	)

	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]string{
		"code":        "authcode",
		"phone_id":    "106540352242922",
		"business_id": "524126980791429",
		"name":        "ES Test Line",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateAccountEmbeddedSignup(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var acc handlers.AccountResponse
	testutil.ParseEnvelopeResponse(t, req, &acc)
	assert.Equal(t, "106540352242922", acc.PhoneID)
	assert.Equal(t, "524126980791429", acc.BusinessID)
	assert.Equal(t, "ES Test Line", acc.Name)
	assert.Equal(t, "appid", acc.AppID)
	assert.True(t, acc.HasAccessToken)
}

func TestApp_CreateAccountEmbeddedSignup_NotConfigured(t *testing.T) {
	t.Parallel()

	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]string{
		"code": "x", "phone_id": "1", "business_id": "2",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	err := app.CreateAccountEmbeddedSignup(req)
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusNotFound, testutil.GetResponseStatusCode(req))
}
