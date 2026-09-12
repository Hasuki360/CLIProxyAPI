package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AntigravityReverseProxyPayload defines the payload for getting or updating the Antigravity reverse proxy config.
type AntigravityReverseProxyPayload struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	Value    *bool   `json:"value,omitempty"`
	BaseURL  *string `json:"base-url,omitempty"`
	TokenURL *string `json:"token-url,omitempty"`
}

// GetAntigravityReverseProxy returns the current Antigravity reverse proxy configuration.
func (h *Handler) GetAntigravityReverseProxy(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"value":     false,
			"base-url":  "https://ag.hasuki.top",
			"token-url": "https://ag.hasuki.top/token",
		})
		return
	}
	enabled := h.cfg.Antigravity.IsReverseProxyEnabled()
	baseURL := h.cfg.Antigravity.ReverseProxyBaseURL()
	tokenURL := h.cfg.Antigravity.ReverseProxyTokenURL()
	c.JSON(http.StatusOK, gin.H{
		"enabled":   enabled,
		"value":     enabled,
		"base-url":  baseURL,
		"token-url": tokenURL,
	})
}

// PutAntigravityReverseProxy updates the Antigravity reverse proxy configuration.
func (h *Handler) PutAntigravityReverseProxy(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler_unavailable"})
		return
	}
	var body AntigravityReverseProxyPayload
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	enabled := body.Enabled
	if enabled == nil && body.Value != nil {
		enabled = body.Value
	}
	if enabled != nil {
		h.cfg.Antigravity.ReverseProxy.Enabled = enabled
	}
	if body.BaseURL != nil {
		trimmed := strings.TrimRight(strings.TrimSpace(*body.BaseURL), "/")
		h.cfg.Antigravity.ReverseProxy.BaseURL = trimmed
	}
	if body.TokenURL != nil {
		trimmed := strings.TrimSpace(*body.TokenURL)
		h.cfg.Antigravity.ReverseProxy.TokenURL = trimmed
	}
	h.persist(c)
}
