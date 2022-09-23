package api

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/session"
)

const loginEndPoint = "/login"

const (
	tripwireActivatedErrResponse = "Stash is exposed to the public internet without authentication, and is not serving any more content to protect your privacy. " +
		"More information and fixes are available at https://github.com/stashapp/stash/wiki/Authentication-Required-When-Accessing-Stash-From-the-Internet"

	tripwireActivatedErrMsg = "Blocked incoming request - Stash is exposed to the public internet without authentication. " +
		"More information and fixes are available at https://github.com/stashapp/stash/wiki/Authentication-Required-When-Accessing-Stash-From-the-Internet"

	externalAccessErrMsg = "You have attempted to access Stash over the internet, and authentication is not enabled. " +
		"This is extremely dangerous! The whole world can see your your stash page and browse your files! " +
		"Stash is not answering any other requests to protect your privacy. " +
		"Please read the log entry or visit https://github.com/stashapp/stash/wiki/Authentication-Required-When-Accessing-Stash-From-the-Internet"
)

func allowUnauthenticated(r *http.Request) bool {
	// #2715 - allow access to UI files
	return strings.HasPrefix(r.URL.Path, loginEndPoint) || r.URL.Path == "/css" || strings.HasPrefix(r.URL.Path, "/assets")
}

func authenticateHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := config.GetInstance()

			if checkSecurityTripwireActivated(c) {
				logger.Warn(tripwireActivatedErrMsg)
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(tripwireActivatedErrResponse))
				return
			}

			userID, err := manager.GetInstance().SessionStore.Authenticate(w, r)
			if err != nil {
				if errors.Is(err, session.ErrInvalidApiKey) {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(err.Error()))
					return
				}

				// unauthorized error
				w.Header().Add("WWW-Authenticate", `FormBased`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if err := session.CheckAllowPublicWithoutAuth(c, r); err != nil {
				var externalAccess session.ExternalAccessError
				switch {
				case errors.As(err, &externalAccess):
					activateSecurityTripwire(c, externalAccess)
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(externalAccessErrMsg))
					return
				default:
					logger.Errorf("Error checking external access security: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			ctx := r.Context()

			if c.HasCredentials() {
				// authentication is required
				if userID == "" && !allowUnauthenticated(r) {
					// authentication was not received, redirect
					// if graphql was requested, we just return a forbidden error
					if r.URL.Path == "/graphql" {
						w.Header().Add("WWW-Authenticate", `FormBased`)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					prefix := getProxyPrefix(r.Header)

					// otherwise redirect to the login page
					u := url.URL{
						Path: prefix + "/login",
					}
					q := u.Query()
					q.Set(returnURLParam, prefix+r.URL.Path)
					u.RawQuery = q.Encode()
					http.Redirect(w, r, u.String(), http.StatusFound)
					return
				}
			}

			ctx = session.SetCurrentUserID(ctx, userID)

			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

func checkSecurityTripwireActivated(c *config.Instance) bool {
	accessErr := session.CheckExternalAccessTripwire(c)
	return accessErr != nil
}

func activateSecurityTripwire(c *config.Instance, accessErr session.ExternalAccessError) {
	session.LogExternalAccessError(accessErr)

	err := c.ActivatePublicAccessTripwire(net.IP(accessErr).String())
	if err != nil {
		logger.Errorf("Error activating public access tripwire: %v", err)
	}
}
