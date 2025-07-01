package auth

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
	"github.com/asadbekGo/ett-code-sdk/tools/auth0"
	"github.com/asadbekGo/ett-code-sdk/tools/beeline"
	"github.com/asadbekGo/ett-code-sdk/tools/click"
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
	Tenant                string `json:"tenant,omitempty"`
	AuthId                string `json:"authId,omitempty"`
}

type Widget struct {
	AuthorizationServices []struct {
		Audience                    string   `json:"audience"`
		AuthID                      string   `json:"auth_id"`
		AuthName                    string   `json:"auth_name"`
		AuthRedirectUrl             string   `json:"auth_redirect_url"`
		ClientID                    string   `json:"client_id"`
		ClientSecretCopy            string   `json:"client_secret_copy"`
		Domain                      string   `json:"domain"`
		GUID                        string   `json:"guid"`
		ManagementClientID          string   `json:"management_client_id"`
		ManagementClientSecretCopy  string   `json:"management_client_secret_copy"`
		ManagementDomain            string   `json:"management_domain"`
		Provider                    []string `json:"provider"`
		Tenant                      string   `json:"tenant"`
		TestCreateSftpServerDisable bool     `json:"test-create-sftp-server_disable"`
		Token                       string   `json:"token"`
		TokenExpireAt               string   `json:"token_expire_at"`
	} `json:"authorization_services"`
}

func GetUserByAccessToken(widgetObject map[string]interface{}, accessToken, secretKey string, ettUcodeApi *sdk.ObjectFunction) (userAccount UserAccount, errorResponse sdk.ResponseError) {

	// Marshal widget object to JSON ...
	body, err := json.Marshal(widgetObject)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to marshal widget object:", err.Error())
		return userAccount, errorResponse
	}

	var widget Widget
	err = json.Unmarshal(body, &widget)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Failed to unmarshal widget object:", err.Error())
		return userAccount, errorResponse
	}

	// Authorization service request ...
	if len(widget.AuthorizationServices) <= 0 {
		errorResponse.StatusCode = 404
		errorResponse.ClientErrorMessage = "Authorization service not found"
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Authorization service not found")
		return userAccount, errorResponse
	}

	var (
		noAvailableAuthRedirectUrlHeaderValue = `Bearer realm="EasyToTravel", error="invalid_token", error_description="Access token invalid"`
		authRedirectUrlHeaderValue            = noAvailableAuthRedirectUrlHeaderValue + `, authRedirectUrl="%s"`
		authorizationService                  = widget.AuthorizationServices[0]
	)
	errorResponse.ResponseHeader = make(map[string]interface{})

	if len(authorizationService.Provider) <= 0 {
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
		if authorizationService.AuthRedirectUrl != "" {
			errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
		}
		return userAccount, errorResponse
	}

	switch authorizationService.Provider[0] {
	case "auth0":
		// Auth0 request ...
		var claims auth0.CustomClaims
		claims, errorResponse = auth0.Auth0ValidateToken(
			auth0.ValidateTokenRequest{
				Domain:      authorizationService.Domain,
				Audience:    authorizationService.Audience,
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
		var isAuthorizationUpdateToken = true
		if authorizationService.TokenExpireAt != "" {
			expireTime, err := time.Parse(time.RFC3339, authorizationService.TokenExpireAt)
			if err != nil {
				errorResponse.StatusCode = 500
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				return userAccount, errorResponse
			}

			isAuthorizationUpdateToken = false
			accessToken = authorizationService.Token
			if !time.Now().UTC().Before(expireTime.Add(-time.Hour * 1)) {
				isAuthorizationUpdateToken = true
			}
		}

		if isAuthorizationUpdateToken {
			var tokenResponse auth0.TokenResponse
			authorizationService.ManagementClientSecretCopy, err = sdk.Decrypt(secretKey, authorizationService.ManagementClientSecretCopy)
			if err != nil {
				go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt auth0 management client secret:", err.Error()))
			}

			tokenResponse, errorResponse = auth0.Auth0GetToken(
				auth0.Credential{
					Domain:       authorizationService.ManagementDomain,
					Audience:     authorizationService.ManagementDomain + "/api/v2/",
					ClientId:     authorizationService.ManagementClientID,
					ClientSecret: authorizationService.ManagementClientSecretCopy,
					GrantType:    "client_credentials",
				},
				ettUcodeApi,
			)
			if errorResponse.ErrorMessage != "" {
				return userAccount, errorResponse
			}

			tokenExpiretat := time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn)).Format(time.RFC3339)

			var updateAuthorizationServiceRequest = sdk.Request{Data: map[string]interface{}{"guid": authorizationService.GUID, "token": tokenResponse.AccessToken, "token_expire_at": tokenExpiretat}}
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
				Domain:      authorizationService.Domain,
				Audience:    authorizationService.Domain + "/api/v2/",
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
		userAccount = UserAccount{ExternalUserId: claims.RegisteredClaims.Sub, FirstName: users[0].Name, Email: users[0].Email, Phone: users[0].PhoneNumber}
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
		authorizationService.ClientSecretCopy, err = sdk.Decrypt(secretKey, authorizationService.ClientSecretCopy)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt beeline client secret:", err.Error()))
		}

		userID, err := beeline.ValidateToken(authorizationService.ClientSecretCopy, accessToken)
		if err != nil {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}
		if userID == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("User id is required")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		beelineUserAccount, err := beeline.GetUser(
			beeline.GetUsersRequest{
				Domain:      authorizationService.Domain,
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
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if !strings.HasPrefix(beelineUserAccount.PhoneNumber, "+") {
			beelineUserAccount.PhoneNumber = "+" + beelineUserAccount.PhoneNumber
		}

		userAccount = UserAccount{
			ExternalUserId: userID,
			Phone:          beelineUserAccount.PhoneNumber,
			FirstName:      beelineUserAccount.FirstName,
			LastName:       beelineUserAccount.LastName,
		}

	case "ets":
		// Ets request ...
		accessTokenArr := strings.Split(accessToken, "_")
		if len(accessTokenArr) != 2 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		var userID = accessTokenArr[0]
		authorizationService.ClientSecretCopy, err = sdk.Decrypt(secretKey, authorizationService.ClientSecretCopy)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Failed to decrypt ets client secret:", err.Error()))
		}

		var hasher = md5.New()
		hasher.Write([]byte(authorizationService.ClientSecretCopy + userID))
		hashBytes := hasher.Sum(nil)
		hashString := hex.EncodeToString(hashBytes)
		if hashString != accessTokenArr[1] {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}
		userAccount = UserAccount{ExternalUserId: userID, Email: userID}

	case "click":
		// Click request ...
		authorizationService.ClientSecretCopy, err = sdk.Decrypt(secretKey, authorizationService.ClientSecretCopy)
		if err != nil {
			go ettUcodeApi.SendTelegram(ettUcodeApi.Logger.ErrorLog.Sprint("Error while decrypting click clientSecret " + err.Error()))
		}

		clickUserAccount, err := click.GetUser(
			click.GetUsersRequest{
				Domain:      authorizationService.Domain,
				AccessToken: authorizationService.ClientSecretCopy,
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
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if clickUserAccount.Result.ClientId == 0 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Client id is required")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		userAccount = UserAccount{
			ExternalUserId: strconv.Itoa(clickUserAccount.Result.ClientId),
			Phone:          clickUserAccount.Result.PhoneNumber,
			FirstName:      clickUserAccount.Result.Name,
			LastName:       clickUserAccount.Result.Surname,
		}

	case "schmetterling":
		tokenArr := strings.Split(accessToken, "_")
		if len(tokenArr) != 2 {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		if tokenArr[0] == "" {
			errorResponse.StatusCode = 401
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Invalid accessToken")
			errorResponse.ResponseHeader["WWW-Authenticate"] = noAvailableAuthRedirectUrlHeaderValue
			if authorizationService.AuthRedirectUrl != "" {
				errorResponse.ResponseHeader["WWW-Authenticate"] = fmt.Sprintf(authRedirectUrlHeaderValue, authorizationService.AuthRedirectUrl)
			}
			return userAccount, errorResponse
		}

		userAccount = UserAccount{
			ExternalAgencyId: tokenArr[0],
			ExternalUserId:   tokenArr[1],
			Email:            tokenArr[1],
		}

	default:
		errorResponse.StatusCode = 404
		errorResponse.ClientErrorMessage = "Authorization provider not found"
		errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint("Authorization provider not found")
		return userAccount, errorResponse
	}

	userAccount.AuthorizationProvider = authorizationService.Provider[0]
	userAccount.Tenant = authorizationService.Tenant
	userAccount.AuthId = authorizationService.AuthID

	return userAccount, errorResponse
}
