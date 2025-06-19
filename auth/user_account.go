package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
	"github.com/asadbekGo/ett-code-sdk/tools/auth0"
	"github.com/asadbekGo/ett-code-sdk/tools/beeline"
	"github.com/asadbekGo/ett-code-sdk/tools/click"
	"github.com/spf13/cast"
)

type UserAccount struct {
	Id                    string `json:"id,omitempty"`
	ExternalUserId        string `json:"external_user_id,omitempty"`
	ExternalAgencyId      string `json:"external_agency_id,omitempty"`
	FirstName             string `json:"firstName,omitempty"`
	LastName              string `json:"lastName,omitempty"`
	Email                 string `json:"email,omitempty"`
	Phone                 string `json:"phone,omitempty"`
	Photo                 string `json:"photo,omitempty"`
	SelectedLanguage      string `json:"selectedLanguage,omitempty"`
	SelectedCurrency      string `json:"selectedCurrency,omitempty"`
	AuthorizationProvider string `json:"authorizationProvider,omitempty"`
}

func GetUserByAccessToken(widgetObject map[string]interface{}, accessToken, secretKey string, ettUcodeApi *sdk.ObjectFunction) (userAccount UserAccount, errorResponse sdk.ResponseError) {

	// Authorization service request ...
	var authorizationServices = cast.ToSlice(widgetObject["authorization_services"])
	if len(authorizationServices) <= 0 {
		errorResponse.StatusCode = 404
		errorResponse.ClientErrorMessage = "Authorization service not found"
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Authorization service not found")
		return userAccount, errorResponse
	}

	var (
		noAvailableAuthRedirectUrlHeaderValue = `Bearer realm="EasyToTravel", error="invalid_token", error_description="Access token invalid"`
		authRedirectUrlHeaderValue            = noAvailableAuthRedirectUrlHeaderValue + `, authRedirectUrl="%s"`
		authorizationServiceObject            = cast.ToStringMap(authorizationServices[0])
		authorizationProvider                 = cast.ToStringSlice(authorizationServiceObject["provider"])
		authRedirectUrl                       = cast.ToString(authorizationServiceObject["auth_redirect_url"])
	)
	errorResponse.ResponseHeader = make(map[string]interface{})

	if len(authorizationProvider) <= 0 {
		errorResponse.StatusCode = 404
		errorResponse.ClientErrorMessage = "Authorization provider not found"
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Authorization provider not found")
		return userAccount, errorResponse
	}

	if accessToken == "" {
		errorResponse.StatusCode = 401
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Token is required")
		errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
		if authRedirectUrl != "" {
			errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
		}
		return userAccount, errorResponse
	}

	switch authorizationProvider[0] {
	case "auth0":
		// Auth0 request ...
		var claims auth0.CustomClaims
		claims, errorResponse = auth0.Auth0ValidateToken(
			auth0.ValidateTokenRequest{
				Domain:      cast.ToString(authorizationServiceObject["domain"]),
				Audience:    cast.ToString(authorizationServiceObject["audience"]),
				AccessToken: accessToken,
			},
			ettUcodeApi,
		)
		if errorResponse.ErrorMessage != "" {
			return userAccount, errorResponse
		}

		if claims.RegisteredClaims.Sub == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Empty sub")
			return userAccount, errorResponse
		}

		// Auth0 get token ...
		var (
			authorizationTokenExpireAt = cast.ToString(authorizationServiceObject["token_expire_at"])
			isAuthorizationUpdateToken = true
		)

		if authorizationTokenExpireAt != "" {
			expireTime, err := time.Parse(time.RFC3339, authorizationTokenExpireAt)
			if err != nil {
				errorResponse.StatusCode = 500
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				return userAccount, errorResponse
			}

			isAuthorizationUpdateToken = false
			accessToken = cast.ToString(authorizationServiceObject["token"])
			if !time.Now().UTC().Before(expireTime.Add(-time.Hour * 1)) {
				isAuthorizationUpdateToken = true
			}
		}

		if isAuthorizationUpdateToken {
			var (
				tokenResponse          auth0.TokenResponse
				managementClientSecret = cast.ToString(authorizationServiceObject["management_client_secret_copy"])
			)

			managementClientSecret, err := sdk.Decrypt(secretKey, managementClientSecret)
			if err != nil {
				go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt auth0 management client secret:", err.Error()))
				managementClientSecret = cast.ToString(authorizationServiceObject["management_client_secret_copy"])
			}

			tokenResponse, errorResponse = auth0.Auth0GetToken(
				auth0.Credential{
					Domain:       cast.ToString(authorizationServiceObject["management_domain"]),
					Audience:     cast.ToString(authorizationServiceObject["management_domain"]) + "/api/v2/",
					ClientId:     cast.ToString(authorizationServiceObject["management_client_id"]),
					ClientSecret: managementClientSecret,
					GrantType:    "client_credentials",
				},
				ettUcodeApi,
			)
			if errorResponse.ErrorMessage != "" {
				return userAccount, errorResponse
			}

			tokenExpiretat := time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn)).Format(time.RFC3339)

			var updateAuthorizationServiceRequest = sdk.Request{Data: map[string]interface{}{"guid": authorizationServiceObject["guid"], "token": tokenResponse.AccessToken, "token_expire_at": tokenExpiretat}}
			_, response, err := ettUcodeApi.UpdateObject(&sdk.Argument{TableSlug: "authorization_services", Request: updateAuthorizationServiceRequest, BlockBuilder: true, DisableFaas: true})
			if err != nil {
				errorResponse.StatusCode = 500
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				return userAccount, errorResponse
			}
			accessToken = tokenResponse.AccessToken
		}

		// Auth0 get users ...
		var users []auth0.User
		users, errorResponse = auth0.Auth0GetUsers(
			auth0.GetUsersRequest{
				Domain:      cast.ToString(authorizationServiceObject["domain"]),
				Audience:    cast.ToString(authorizationServiceObject["domain"]) + "/api/v2/",
				Sub:         claims.RegisteredClaims.Sub,
				AccessToken: accessToken,
			},
			ettUcodeApi,
		)
		if errorResponse.ErrorMessage != "" {
			return userAccount, errorResponse
		}

		if len(users) <= 0 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("User not found")
			return userAccount, errorResponse
		}

		var fullName = strings.Split(users[0].Name, " ")
		userAccount = UserAccount{ExternalUserId: claims.RegisteredClaims.Sub, FirstName: users[0].Name, Email: users[0].Email, Phone: users[0].PhoneNumber, AuthorizationProvider: authorizationProvider[0]}
		if len(fullName) == 2 {
			userAccount.FirstName = fullName[0]
			userAccount.LastName = fullName[1]
		}

		for _, provider := range users[0].Identities {
			switch provider.Provider {
			case "google-oauth2":
				if strings.HasSuffix(users[0].Picture, "-c") {
					// Replace "-c" with "0"
					users[0].Picture = strings.TrimSuffix(users[0].Picture, "-c") + "0"
				}

				userAccount.Photo = users[0].Picture
			default:
				userAccount.Photo = users[0].Picture
				if users[0].PictureLarge != "" {
					userAccount.Photo = users[0].PictureLarge
				}
			}
		}

	case "beeline":
		// Beeline request ...
		var clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		clientSecret, err := sdk.Decrypt(secretKey, clientSecret)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt beeline client secret:", err.Error()))
			clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		}

		userID, err := beeline.ValidateToken(clientSecret, accessToken)
		if err != nil {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}
		if userID == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("User id is required")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		beelineUserAccount, err := beeline.GetUser(
			beeline.GetUsersRequest{
				Domain:      cast.ToString(authorizationServiceObject["domain"]),
				AccessToken: accessToken,
			},
		)

		if err != nil {
			errorResponse.StatusCode = 500
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			return userAccount, errorResponse
		}

		if beelineUserAccount.PhoneNumber == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Phone number is required")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if !strings.HasPrefix(beelineUserAccount.PhoneNumber, "+") {
			beelineUserAccount.PhoneNumber = "+" + beelineUserAccount.PhoneNumber
		}

		userAccount = UserAccount{
			ExternalUserId:        userID,
			Phone:                 beelineUserAccount.PhoneNumber,
			FirstName:             beelineUserAccount.FirstName,
			LastName:              beelineUserAccount.LastName,
			AuthorizationProvider: authorizationProvider[0],
		}

	case "ets":
		// Ets request ...
		accessTokenArr := strings.Split(accessToken, "_")
		if len(accessTokenArr) != 2 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		var (
			userID       = accessTokenArr[0]
			clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		)

		clientSecret, err := sdk.Decrypt(secretKey, clientSecret)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt ets client secret:", err.Error()))
			clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		}

		var hasher = md5.New()
		hasher.Write([]byte(clientSecret + userID))
		hashBytes := hasher.Sum(nil)
		hashString := hex.EncodeToString(hashBytes)
		if hashString != accessTokenArr[1] {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}
		userAccount = UserAccount{ExternalUserId: userID, Email: userID, AuthorizationProvider: authorizationProvider[0]}

	case "click":
		// Click request ...
		var clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		clientSecret, err := sdk.Decrypt(secretKey, clientSecret)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Error while decrypting click clientSecret " + err.Error()))
			clientSecret = cast.ToString(authorizationServiceObject["client_secret_copy"])
		}

		clickUserAccount, err := click.GetUser(
			click.GetUsersRequest{
				Domain:      cast.ToString(authorizationServiceObject["domain"]),
				AccessToken: clientSecret,
				WebSession:  accessToken,
			},
		)
		if err != nil {
			errorResponse.StatusCode = 500
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			return userAccount, errorResponse
		}

		if clickUserAccount.Error.Code != 0 || clickUserAccount.Error.Message != "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(clickUserAccount.Error.Message)
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if clickUserAccount.Result.ClientId == 0 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Client id is required")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		userAccount = UserAccount{
			ExternalUserId:        strconv.Itoa(clickUserAccount.Result.ClientId),
			Phone:                 clickUserAccount.Result.PhoneNumber,
			FirstName:             clickUserAccount.Result.Name,
			LastName:              clickUserAccount.Result.Surname,
			AuthorizationProvider: authorizationProvider[0],
		}

	case "schmetterling":
		tokenArr := strings.Split(accessToken, "_")
		if len(tokenArr) != 2 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if tokenArr[0] == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authRedirectUrl)
			}
			return userAccount, errorResponse
		}

		userAccount = UserAccount{
			ExternalAgencyId:      tokenArr[0],
			ExternalUserId:        tokenArr[1],
			Email:                 tokenArr[1],
			AuthorizationProvider: authorizationProvider[0],
		}

	default:
		errorResponse.StatusCode = 404
		errorResponse.ClientErrorMessage = "Authorization provider not found"
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Authorization provider not found")
		return userAccount, errorResponse
	}

	return userAccount, errorResponse
}
