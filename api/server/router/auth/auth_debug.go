package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

// debugToken handles the /auth/token/debug endpoint
func (r *authRouter) debugToken(w http.ResponseWriter, req *http.Request, vars map[string]string) error {
	if err := req.ParseForm(); err != nil {
		return err
	}

	tokenStr := req.Form.Get("token")
	if tokenStr == "" {
		return errors.New("missing token parameter")
	}

	// Parse JWT without verification (for debugging only)
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return errors.New("invalid token format: token should have three dot-separated parts")
	}

	// Decode the payload (second part of the token)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.Wrap(err, "failed to decode token payload")
	}

	// Parse the JSON payload
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return errors.Wrap(err, "failed to parse token claims")
	}

	// Return token claims as JSON
	response := types.AuthTokenDebugResponse{
		Claims: claims,
	}

	return httputils.WriteJSON(w, http.StatusOK, response)
}
