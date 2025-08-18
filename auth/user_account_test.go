package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	sdk "github.com/asadbekGo/ett-code-sdk"
	"github.com/stretchr/testify/assert"
)

type (
	getUserByAccessTokenTestCase struct {
		name                  string
		args                  args
		expectedResponse      UserAccount
		expectedErrorResponse *sdk.ResponseError
		expectError           bool
	}

	args struct {
		accessToken  string
		secretKey    string
		ettUcodeApi  *sdk.ObjectFunction
		widgetObject map[string]interface{}
	}
)

var getUserByAccessTokenTestCases = []getUserByAccessTokenTestCase{
	{
		name: "schmetterling success case",
		args: args{
			accessToken: "mirabbos_botirjonov",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{Provider: []string{"schmetterling"}}},
			},
		},
		expectedResponse: UserAccount{
			ExternalAgencyId:      "mirabbos",
			ExternalUserId:        "botirjonov",
			Email:                 "botirjonov",
			AuthorizationProvider: "schmetterling",
		},
		expectedErrorResponse: nil,
		expectError:           false,
	},
	{
		name: "schmetterling error case 1",
		args: args{
			accessToken: "invalid_access_token",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{Provider: []string{"schmetterling"}}},
			},
		},
		expectedResponse: UserAccount{},
		expectedErrorResponse: &sdk.ResponseError{
			StatusCode:         401,
			ClientErrorMessage: sdk.ErrorCodeWithMessage[401],
			ResponseHeader: map[string]interface{}{
				"WWW-Authenticate": noAvailableAuthRedirectUrlHeaderValue,
			},
		},
		expectError: true,
	},
	{
		name: "schmetterling error case 2",
		args: args{
			accessToken: "_g",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{Provider: []string{"schmetterling"}}},
			},
		},
		expectedResponse: UserAccount{},
		expectedErrorResponse: &sdk.ResponseError{
			StatusCode:         401,
			ClientErrorMessage: sdk.ErrorCodeWithMessage[401],
			ResponseHeader: map[string]interface{}{
				"WWW-Authenticate": noAvailableAuthRedirectUrlHeaderValue,
			},
		},
		expectError: true,
	},
	{
		name: "schmetterling error case 3",
		args: args{
			accessToken: "_",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{Provider: []string{"schmetterling"}}},
			},
		},
		expectedResponse: UserAccount{},
		expectedErrorResponse: &sdk.ResponseError{
			StatusCode:         401,
			ClientErrorMessage: sdk.ErrorCodeWithMessage[401],
			ResponseHeader: map[string]interface{}{
				"WWW-Authenticate": noAvailableAuthRedirectUrlHeaderValue,
			},
		},
		expectError: true,
	},
	// {
	// 	name: "ets success case",
	// 	args: args{
	// 		accessToken: "67890_6df0c6c348bf74f4b02ebdd25d320f89",
	// 		secretKey:   "",
	// 		ettUcodeApi: &sdk.ObjectFunction{},
	// 		widgetObject: map[string]interface{}{
	// 			"authorization_services": []AuthorizationService{{
	// 				Provider:         []string{"ets"},
	// 				ClientSecretCopy: "test_client_secret",
	// 			}},
	// 		},
	// 	},
	// 	expectedResponse: UserAccount{
	// 		ExternalUserId:        "67890",
	// 		Email:                 "67890",
	// 		AuthorizationProvider: "ets",

	// 	},
	// 	expectedErrorResponse: nil,
	// 	expectError:           false,
	// },
	{
		name: "ets error case 1",
		args: args{
			accessToken: "67890_6df0c6c348bf74f4b02ebdd25d320f88",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{
					Provider:         []string{"ets"},
					ClientSecretCopy: "test_client_secret",
				}},
			},
		},
		expectedResponse: UserAccount{},
		expectedErrorResponse: &sdk.ResponseError{
			StatusCode:         401,
			ClientErrorMessage: sdk.ErrorCodeWithMessage[401],
			ResponseHeader: map[string]interface{}{
				"WWW-Authenticate": noAvailableAuthRedirectUrlHeaderValue,
			},
		},
		expectError: true,
	},
	{
		name: "ets error case 2",
		args: args{
			accessToken: "67890",
			secretKey:   "",
			ettUcodeApi: &sdk.ObjectFunction{},
			widgetObject: map[string]interface{}{
				"authorization_services": []AuthorizationService{{
					Provider:         []string{"ets"},
					ClientSecretCopy: "test_client_secret",
				}},
			},
		},
		expectedResponse: UserAccount{},
		expectedErrorResponse: &sdk.ResponseError{
			StatusCode:         401,
			ClientErrorMessage: sdk.ErrorCodeWithMessage[401],
			ResponseHeader: map[string]interface{}{
				"WWW-Authenticate": noAvailableAuthRedirectUrlHeaderValue,
			},
		},
		expectError: true,
	},
	// {
	// 	name: "case 1",
	// 	args: args{
	// 		accessToken: "mirabbos_botirjonov",
	// 		secretKey:   "",
	// 		ettUcodeApi: &sdk.ObjectFunction{},
	// 		widgetObject: map[string]interface{}{
	// 			"authorization_services": []AuthorizationService{
	// 				{
	// 					Audience:                    "test_audience",
	// 					AuthID:                      "test_auth_id",
	// 					AuthName:                    "test_auth_name",
	// 					AuthRedirectUrl:             "https://test.auth0.com/redirect",
	// 					ClientID:                    "test_client_id",
	// 					ClientSecretCopy:            "test_client_secret",
	// 					Domain:                      "test.auth0.com",
	// 					GUID:                        "test_guid",
	// 					ManagementClientID:          "test_management_client_id",
	// 					ManagementClientSecretCopy:  "test_management_client_secret",
	// 					ManagementDomain:            "test.management.auth0.com",
	// 					Provider:                    []string{"schmetterling"},
	// 					Tenant:                      "test_tenant",
	// 					TestCreateSftpServerDisable: false,
	// 					Token:                       "test_token",
	// 					TokenExpireAt:               time.Now().Add(time.Hour).Format(time.RFC3339),
	// 				},
	// 			},
	// 		},
	// 	},
	// 	expectedResponse: UserAccount{
	// 		Email:     "test@example.com",
	// 		FirstName: "Test",
	// 	},
	// },
}

var (
	noAvailableAuthRedirectUrlHeaderValue = `Bearer realm="EasyToTravel", error="invalid_token", error_description="Access token invalid"`
	sdkObj                                = sdk.New(&sdk.Config{})
)

func TestGetUserByAccessToken(t *testing.T) {

	for _, tc := range getUserByAccessTokenTestCases {
		t.Run(tc.name, func(t *testing.T) {
			userAccount, errorResponse := GetUserByAccessToken(tc.args.widgetObject, tc.args.accessToken, tc.args.secretKey, sdkObj)
			if tc.expectError {
				assert.NotEmpty(t, errorResponse.ErrorMessage, "expected error but got none")
				assert.Equal(t, UserAccount{}, userAccount, "expected empty user account on error")
				if tc.expectedErrorResponse != nil {
					assert.Equal(t, tc.expectedErrorResponse.StatusCode, errorResponse.StatusCode)
					assert.Equal(t, tc.expectedErrorResponse.ClientErrorMessage, errorResponse.ClientErrorMessage)
					assert.Equal(t, tc.expectedErrorResponse.ResponseHeader["WWW-Authenticate"], errorResponse.ResponseHeader["WWW-Authenticate"])
				}
			} else {
				assert.Empty(t, errorResponse.ErrorMessage, "unexpected error occurred")
				assert.Equal(t, tc.expectedResponse, userAccount, "user account mismatch")
			}
		})
	}
	t.Run("auth0 dynamic test", func(t *testing.T) {
		// assert.NotEmpty(t, "", "errrrrr")

		accessToken, err := getAuth0AccessToken()
		if err != nil {
			t.Skipf("skipping auth0 test due to error: %v", err)
		}

		domain := os.Getenv("AUTH0_DOMAIN")
		clientID := os.Getenv("AUTH0_CLIENT_ID")
		clientSecret := os.Getenv("AUTH0_CLIENT_SECRET")
		audience := os.Getenv("AUTH0_AUDIENCE")
		managementDomain := os.Getenv("AUTH0_MANAGEMENT_DOMAIN")
		managementClientID := os.Getenv("AUTH0_MANAGEMENT_CLIENT_ID")
		managementClientSecret := os.Getenv("AUTH0_MANAGEMENT_CLIENT_SECRET")

		domain = "dev-gyop0d17cyhgd4uo.us.auth0.com"
		clientID = "g446ATGhalhEkAtCgnSqffYVLKFkeyeQ"
		clientSecret = "JkW6DMI-oaifnlA9iJDlr_mEhVxuDmG2EFrN0_g-vzLGALB1OE5H-UIf6p2-IVLw"
		audience = "/api/v1/agent/widget"
		managementDomain = "dev-gyop0d17cyhgd4uo.us.auth0.com"
		managementClientID = "ZiRzlm9xR2G419eLTmP8r654VYF3TCgD"
		managementClientSecret = "57SesJCs23kUuv4ODnnpLWSaYYTV_EVe09JvvjZTxH0Q8UCTYOm4s7RZzcAFWUDQ"

		userAccount, errorResponse := GetUserByAccessToken(map[string]interface{}{
			"authorization_services": []AuthorizationService{
				{
					Provider:                   []string{"auth0"},
					Domain:                     domain,
					Audience:                   audience,
					ClientID:                   clientID,
					ClientSecretCopy:           clientSecret,
					ManagementDomain:           managementDomain,
					ManagementClientID:         managementClientID,
					ManagementClientSecretCopy: managementClientSecret,
				},
			},
		}, accessToken, "", sdkObj)

		assert.Empty(t, errorResponse.ErrorMessage, "unexpected error from Auth0")
		assert.NotEmpty(t, userAccount.Email, "expected a valid user email")
		t.Logf("auth0 success user: %+v", userAccount)
	})

}
func getAuth0AccessToken() (string, error) {
	domain := os.Getenv("AUTH0_DOMAIN")
	clientID := os.Getenv("AUTH0_CLIENT_ID")
	clientSecret := os.Getenv("AUTH0_CLIENT_SECRET")
	audience := os.Getenv("AUTH0_AUDIENCE")

	domain = "dev-gyop0d17cyhgd4uo.us.auth0.com"
	clientID = "g446ATGhalhEkAtCgnSqffYVLKFkeyeQ"
	clientSecret = "JkW6DMI-oaifnlA9iJDlr_mEhVxuDmG2EFrN0_g-vzLGALB1OE5H-UIf6p2-IVLw"
	audience = "/api/v1/agent/widget"

	if domain == "" || clientID == "" || clientSecret == "" || audience == "" {
		return "", fmt.Errorf("missing Auth0 env variables")
	}

	url := fmt.Sprintf("https://%s/oauth/token", domain)

	payload := map[string]string{
		"grant_type":    "password",
		"username":      "test_user_for_auth_sdk@easyto.travel",
		"password":      "test_user_for_auth_sdk@easyto.travel1",
		"audience":      audience,
		"scope":         "read:widget write:payment:create write:payment:complete",
		"client_id":     "g446ATGhalhEkAtCgnSqffYVLKFkeyeQ",
		"client_secret": "JkW6DMI-oaifnlA9iJDlr_mEhVxuDmG2EFrN0_g-vzLGALB1OE5H-UIf6p2-IVLw",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth0 token error: %s", respBody)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}
