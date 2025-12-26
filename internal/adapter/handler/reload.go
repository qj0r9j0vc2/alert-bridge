package handler

import (
	"net/http"

	"github.com/qj0r9j0vc2/alert-bridge/internal/domain/logger"
	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/config"
)

// ReloadHandler handles configuration reload requests.
type ReloadHandler struct {
	configManager *config.ConfigManager
	logger        logger.Logger
}

// NewReloadHandler creates a new reload handler.
func NewReloadHandler(cm *config.ConfigManager, logger logger.Logger) *ReloadHandler {
	return &ReloadHandler{
		configManager: cm,
		logger:        logger,
	}
}

// ServeHTTP handles POST /-/reload requests.
func (h *ReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.configManager.TryReload(); err != nil {
		if err == config.ErrRequiresRestart {
			// Static config change - log warning but return 200
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Configuration change requires restart\n"))
			return
		}

		// Reload failed - return error
		h.logger.Error("manual reload failed", "error", err)
		http.Error(w, "Configuration reload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Configuration reloaded successfully\n"))
}
