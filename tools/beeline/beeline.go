package beeline

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cast"
)

func getPublicKeyFromPEM(publicKey string) (*rsa.PublicKey, error) {
	publicKey = strings.ReplaceAll(publicKey, " ", "\n")
	publicKey = "-----BEGIN PUBLIC KEY-----\n" + publicKey + "\n-----END PUBLIC KEY-----"
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("failed to parse PEM block containing the public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DER-encoded public key: %v", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPubKey, nil
}

func ValidateToken(publicKey, tokenString string) (string, error) {

	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	pubKey, err := getPublicKeyFromPEM(publicKey)
	if err != nil {
		return "", errors.New("Failed to get public key from PEM: " + err.Error())
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	})

	if err != nil || !token.Valid {
		return "", errors.New("Invalid token" + err.Error())
	}

	if _, ok := token.Claims.(jwt.MapClaims); !ok {
		return "", errors.New("failed to cast claims to map")
	}
	claims := token.Claims.(jwt.MapClaims)
	if _, ok := claims["user_id"]; !ok {
		return "", errors.New("user_id not found in claims")
	}
	userID := cast.ToString(claims["user_id"])

	return userID, nil
}

func GetUser(getUsersRequest GetUsersRequest) (GetUsersResponse, error) {

	if !strings.Contains(getUsersRequest.Domain, "https://") {
		getUsersRequest.Domain = "https://" + getUsersRequest.Domain
	}

	request, err := http.NewRequest("GET", getUsersRequest.Domain+"/api/v1/external/integration/user", nil)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to create request" + err.Error())
	}
	request.Header.Add("token", getUsersRequest.AccessToken)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	response, err := client.Do(request)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to get user" + err.Error())
	}

	m, err := json.Marshal(response.Body)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to marshal response" + err.Error())
	}

	if response.StatusCode != 200 {
		return GetUsersResponse{}, errors.New("failed to get user status is not 200" + string(m))
	}

	// parse response
	var getUsersResponse GetUsersResponse

	err = json.NewDecoder(response.Body).Decode(&getUsersResponse)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to decode response" + err.Error())
	}

	return getUsersResponse, nil
}

type GetUsersRequest struct {
	Domain      string `json:"domain"`
	AccessToken string `json:"access_token"`
}

type GetUsersResponse struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	MiddleName  string `json:"middle_name"`
	LastName    string `json:"last_name"`
}
