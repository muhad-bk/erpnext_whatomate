package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

func embeddedSignupEnabled(w config.WhatsAppConfig) bool {
	return w.MetaAppID != "" && w.MetaAppSecret != "" && w.EmbeddedSignupConfigID != ""
}

// GetMetaEmbeddedSignupConfig returns public Meta JS SDK settings for Embedded Signup (no secrets).
func (a *App) GetMetaEmbeddedSignupConfig(r *fastglue.Request) error {
	if _, err := a.getOrgID(r); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	w := a.Config.WhatsApp
	en := embeddedSignupEnabled(w)
	appID, configID := "", ""
	if en {
		appID = w.MetaAppID
		configID = w.EmbeddedSignupConfigID
	}
	graphV := w.APIVersion
	if graphV == "" {
		graphV = "v18.0"
	}

	return r.SendEnvelope(map[string]any{
		"enabled":           en,
		"app_id":            appID,
		"config_id":         configID,
		"graph_api_version": graphV,
	})
}

// embeddedSignupCompleteRequest is the payload after the browser completes FB.login + WA_EMBEDDED_SIGNUP.
type embeddedSignupCompleteRequest struct {
	Code       string `json:"code"`
	PhoneID    string `json:"phone_id"`
	BusinessID string `json:"business_id"`
	Name       string `json:"name"`
}

type metaOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type metaAPIErrorBody struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

func (a *App) exchangeMetaOAuthCode(ctx context.Context, code string) (string, error) {
	w := a.Config.WhatsApp
	base := strings.TrimSuffix(w.BaseURL, "/")
	u, err := url.Parse(fmt.Sprintf("%s/%s/oauth/access_token", base, w.APIVersion))
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", w.MetaAppID)
	q.Set("client_secret", w.MetaAppSecret)
	q.Set("code", code)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr metaAPIErrorBody
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("meta oauth error (%d): %s", apiErr.Error.Code, apiErr.Error.Message)
		}
		return "", fmt.Errorf("meta oauth error: status %d: %s", resp.StatusCode, string(body))
	}

	var tok metaOAuthTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("parse oauth response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in oauth response")
	}
	return tok.AccessToken, nil
}

// CreateAccountEmbeddedSignup exchanges the short-lived OAuth code from Embedded Signup and creates a WhatsApp account.
func (a *App) CreateAccountEmbeddedSignup(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if !embeddedSignupEnabled(a.Config.WhatsApp) {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Embedded Signup is not configured", nil, "")
	}

	var req embeddedSignupCompleteRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if strings.TrimSpace(req.Code) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "code is required", nil, "")
	}
	if strings.TrimSpace(req.PhoneID) == "" || strings.TrimSpace(req.BusinessID) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "phone_id and business_id are required (complete Embedded Signup with a phone number)", nil, "")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	accessToken, err := a.exchangeMetaOAuthCode(ctx, strings.TrimSpace(req.Code))
	if err != nil {
		a.Log.Error("Embedded Signup token exchange failed", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to exchange authorization code: "+err.Error(), nil, "")
	}

	w := a.Config.WhatsApp
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("WhatsApp %s", req.PhoneID)
	}

	ar := AccountRequest{
		Name:        name,
		AppID:       w.MetaAppID,
		PhoneID:     strings.TrimSpace(req.PhoneID),
		BusinessID:  strings.TrimSpace(req.BusinessID),
		AccessToken: accessToken,
		APIVersion:  w.APIVersion,
	}
	if w.MetaAppSecret != "" {
		ar.AppSecret = w.MetaAppSecret
	}

	account, err := a.persistNewWhatsAppAccount(orgID, userID, ar)
	if err != nil {
		if errors.Is(err, errInvalidAccountRequest) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid account data", nil, "")
		}
		a.Log.Error("Failed to create account from Embedded Signup", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create account", nil, "")
	}

	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"account", account.ID, models.AuditActionCreated, nil, account)

	return r.SendEnvelope(accountToResponse(*account))
}
