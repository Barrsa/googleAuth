package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/googleAuth/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthHandler struct {
	config     *config.Config
	oauth2Conf *oauth2.Config
	store      cookie.Store
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Locale        string `json:"locale"`
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	oauth2Conf := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	store := cookie.NewStore([]byte(cfg.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 24 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.Environment == "production",
		SameSite: http.SameSiteLaxMode,
	})

	return &AuthHandler{
		config:     cfg,
		oauth2Conf: oauth2Conf,
		store:      store,
	}
}

// GetStore returns the session store for middleware setup
func (h *AuthHandler) GetStore() cookie.Store {
	return h.store
}

// Login initiates the Google OAuth flow
func (h *AuthHandler) Login(c *gin.Context) {
	// Generate state token for CSRF protection
	state := generateStateToken()
	
	// Save state in session
	session := sessions.Default(c)
	session.Set("oauth_state", state)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Redirect to Google OAuth consent page
	url := h.oauth2Conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// Callback handles the OAuth callback from Google
func (h *AuthHandler) Callback(c *gin.Context) {
	// Verify state token
	session := sessions.Default(c)
	storedState := session.Get("oauth_state")
	if storedState == nil || storedState.(string) != c.Query("state") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state parameter"})
		return
	}

	// Clear state from session
	session.Delete("oauth_state")

	// Exchange authorization code for token
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}

	token, err := h.oauth2Conf.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to exchange token: %v", err)})
		return
	}

	// Get user info from Google
	userInfo, err := h.getUserInfo(token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get user info: %v", err)})
		return
	}

	// Save user info and token in session
	session.Set("user", userInfo)
	session.Set("access_token", token.AccessToken)
	if token.RefreshToken != "" {
		session.Set("refresh_token", token.RefreshToken)
	}
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Redirect to frontend or return user info
	c.Redirect(http.StatusTemporaryRedirect, h.config.FrontendURL+"/auth/success")
}

// Profile returns the current user's profile
func (h *AuthHandler) Profile(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// Logout clears the session
func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// getUserInfo fetches user information from Google API
func (h *AuthHandler) getUserInfo(accessToken string) (*GoogleUserInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// generateStateToken generates a random state token for CSRF protection
func generateStateToken() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
