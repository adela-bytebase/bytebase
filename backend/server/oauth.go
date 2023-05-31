package server

import (
	"fmt"
	"net/http"

	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"

	"github.com/bytebase/bytebase/backend/common"
	api "github.com/bytebase/bytebase/backend/legacyapi"
	vcsPlugin "github.com/bytebase/bytebase/backend/plugin/vcs"
)

func (s *Server) registerOAuthRoutes(g *echo.Group) {
	// This is a generic endpoint of exchanging access token for VCS providers. It
	// requires either the "vcsId", "clientId", "clientSecret" to infer the other details
	// from an existing VCS provider or "vcsType", "instanceURL", "clientId" and "clientSecret"
	// to directly compose the request to the VCS host.
	g.POST("/oauth/vcs/exchange-token", func(c echo.Context) error {
		req := &api.VCSExchangeToken{}
		if err := jsonapi.UnmarshalPayload(c.Request().Body, req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Malformed VCS exchange token request").SetInternal(err)
		}

		var vcsType vcsPlugin.Type
		var instanceURL string
		var oauthExchange *common.OAuthExchange
		if req.ID > 0 {
			vcs, err := s.store.GetVCSByID(c.Request().Context(), req.ID)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err)
			}
			if vcs == nil {
				return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to find VCS, ID: %v", req.ID))
			}

			vcsType = vcs.Type
			instanceURL = vcs.InstanceURL
			clientID := req.ClientID
			clientSecret := req.ClientSecret
			// Since we may not pass in ClientID and ClientSecret in the request, we will use the client ID and secret from VCS store even if it's stale.
			// If it's stale, we should return better error messages and ask users to update the VCS secrets.
			// https://sourcegraph.com/github.com/bytebase/bytebase/-/blob/frontend/src/components/RepositorySelectionPanel.vue?L77:8&subtree=true
			// https://github.com/bytebase/bytebase/issues/1372
			if clientID == "" || clientSecret == "" {
				clientID = vcs.ApplicationID
				clientSecret = vcs.Secret
			}
			oauthExchange = &common.OAuthExchange{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         req.Code,
			}
		} else {
			vcsType = req.Type
			if vcsType != vcsPlugin.GitLab && vcsType != vcsPlugin.GitHub && vcsType != vcsPlugin.Bitbucket {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unexpected VCS type: %s", vcsType))
			}

			instanceURL = req.InstanceURL
			oauthExchange = &common.OAuthExchange{
				ClientID:     req.ClientID,
				ClientSecret: req.ClientSecret,
				Code:         req.Code,
			}
		}

		ctx := c.Request().Context()
		setting, err := s.store.GetWorkspaceGeneralSetting(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to find workspace setting").SetInternal(err)
		}

		oauthExchange.RedirectURL = fmt.Sprintf("%s/oauth/callback", setting.ExternalUrl)
		oauthToken, err := vcsPlugin.Get(vcsType, vcsPlugin.ProviderConfig{}).
			ExchangeOAuthToken(
				c.Request().Context(),
				instanceURL,
				oauthExchange,
			)
		if err != nil {
			return echo.NewHTTPError(
				http.StatusInternalServerError,
				fmt.Sprintf("Failed to exchange OAuth token. Make sure %q matches your browser host. Note that if you are not using port 80 or 443, you should also specify the port such as --external-url=http://host:port", setting.ExternalUrl),
			).SetInternal(err)
		}

		resp := &api.OAuthToken{
			AccessToken:  oauthToken.AccessToken,
			RefreshToken: oauthToken.RefreshToken,
			ExpiresTs:    oauthToken.ExpiresTs,
		}
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSONCharsetUTF8)
		if err := jsonapi.MarshalPayload(c.Response().Writer, resp); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to marshal oauth token response").SetInternal(err)
		}
		return nil
	})
}