package auth0

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

func Auth0ValidateToken(validateTokenRequest ValidateTokenRequest, ettUcodeApi *sdk.ObjectFunction) (customClaims CustomClaims, errorResponse sdk.ResponseError) {
	if !strings.Contains(validateTokenRequest.Domain, "https://") {
		validateTokenRequest.Domain = "https://" + validateTokenRequest.Domain + "/"
		// validateTokenRequest.Audience = "https://" + validateTokenRequest.Audience
	}

	if validateTokenRequest.Domain[len(validateTokenRequest.Domain)-1:] != "/" {
		validateTokenRequest.Domain = validateTokenRequest.Domain + "/"
	}

	issuerURL, err := url.Parse(validateTokenRequest.Domain)
	if err != nil {
		errorResponse.StatusCode = 400
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to parse the issuer url: " + err.Error())
		return CustomClaims{}, errorResponse
	}
	var provider = jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	// Create a new validator
	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{validateTokenRequest.Audience},
		validator.WithCustomClaims(
			func() validator.CustomClaims {
				return &CustomClaims{}
			},
		),
	)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to set up the validator: " + err.Error())
		return CustomClaims{}, errorResponse
	}

	// Validate the token
	claims, err := jwtValidator.ValidateToken(context.Background(), validateTokenRequest.AccessToken)
	if err != nil {
		errorResponse.StatusCode = 401
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid token: " + err.Error())
		return CustomClaims{}, errorResponse
	}

	body, err := json.Marshal(claims)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to marshal claims: " + err.Error())
		return CustomClaims{}, errorResponse
	}

	err = json.Unmarshal(body, &customClaims)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to unmarshal claims: " + err.Error())
		return CustomClaims{}, errorResponse
	}

	return customClaims, errorResponse
}

func Auth0GetToken(creds Credential, ettUcodeApi *sdk.ObjectFunction) (tokenResponse TokenResponse, errorResponse sdk.ResponseError) {
	if !strings.Contains(creds.Domain, "https://") {
		creds.Domain = "https://" + creds.Domain + "/"
		creds.Audience = "https://" + creds.Audience
	}

	if creds.Domain[len(creds.Domain)-1:] != "/" {
		creds.Domain = creds.Domain + "/"
	}

	var getTokenRequest = map[string]interface{}{
		"client_id":     creds.ClientId,
		"client_secret": creds.ClientSecret,
		"audience":      creds.Audience,
		"grant_type":    creds.GrantType,
	}

	var headers = map[string]interface{}{"Content-Type": "application/json"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	body, err := sdk.DoRequest(ctx, creds.Domain+"oauth/token", "POST", getTokenRequest, "", headers)
	if err != nil {
		errorResponse.StatusCode = 401
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
		return tokenResponse, errorResponse
	}

	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to unmarshal token response: " + err.Error())
		return tokenResponse, errorResponse
	}

	return tokenResponse, errorResponse
}

func Auth0GetUsers(getUserRequest GetUsersRequest, ettUcodeApi *sdk.ObjectFunction) (users []User, errorResponse sdk.ResponseError) {
	if !strings.Contains(getUserRequest.Domain, "https://") {
		getUserRequest.Domain = "https://" + getUserRequest.Domain + "/"
		getUserRequest.Audience = "https://" + getUserRequest.Audience
	}

	if getUserRequest.Domain[len(getUserRequest.Domain)-1:] != "/" {
		getUserRequest.Domain = getUserRequest.Domain + "/"
	}

	var (
		url     = getUserRequest.Domain + "api/v2/users" + "?search_engine=v3" + "&q=user_id%3D%22" + getUserRequest.Sub + "%22"
		headers = map[string]interface{}{"Authorization": "Bearer " + getUserRequest.AccessToken}
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	body, err := sdk.DoRequest(ctx, url, "GET", nil, "", headers)
	if err != nil {
		errorResponse.StatusCode = 401
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
		return users, errorResponse
	}

	err = json.Unmarshal(body, &users)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to unmarshal users: " + err.Error())
		return users, errorResponse
	}

	return users, errorResponse
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type Credential struct {
	Domain       string `json:"domain"`
	Audience     string `json:"audience"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type GetUsersRequest struct {
	Domain      string `json:"domain"`
	Audience    string `json:"audience"`
	AccessToken string `json:"access_token"`
	Sub         string `json:"sub"`
}

type User struct {
	Identities []struct {
		UserID     string `json:"user_id"`
		Provider   string `json:"provider"`
		IsSocial   bool   `json:"isSocial"`
		Connection string `json:"connection"`
	} `json:"identities"`
	Email         string    `json:"email"`
	Username      string    `json:"username"`
	Nickname      string    `json:"nickname"`
	Picture       string    `json:"picture"`
	PictureLarge  string    `json:"picture_large"`
	PhoneVerified bool      `json:"phone_verified"`
	UpdatedAt     time.Time `json:"updated_at"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	PhoneNumber   string    `json:"phone_number"`
	LastLogin     time.Time `json:"last_login"`
	LastIP        string    `json:"last_ip"`
	LoginsCount   int       `json:"logins_count"`
}

type ValidateTokenRequest struct {
	Domain      string `json:"domain"`
	Audience    string `json:"audience"`
	AccessToken string `json:"access_token"`
}

type CustomClaims struct {
	CustomClaims struct {
		Scope string `json:"scope"`
	} `json:"CustomClaims"`
	RegisteredClaims struct {
		Iss string   `json:"iss"`
		Sub string   `json:"sub"`
		Aud []string `json:"aud"`
		Exp int      `json:"exp"`
		Iat int      `json:"iat"`
	} `json:"RegisteredClaims"`
}

// Validate does nothing for this example, but we need
// it to satisfy validator.CustomClaims interface.
func (c CustomClaims) Validate(ctx context.Context) error {
	return nil
}
