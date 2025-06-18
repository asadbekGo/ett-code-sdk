package click

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	clickErrorMessageWithStatusCode = map[string]int{
		"Сессия прервана. Войдите в приложение заново": 401,
	}
)

func GetUser(getUsersRequest GetUsersRequest) (GetUsersResponse, error) {

	if !strings.Contains(getUsersRequest.Domain, "https://") {
		getUsersRequest.Domain = "https://" + getUsersRequest.Domain
	}

	request, err := http.NewRequest(http.MethodPost, getUsersRequest.Domain+"/integration", strings.NewReader(`{"jsonrpc":"2.0","method":"user.profile","id":126}`))
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to create request" + err.Error())
	}
	request.Header.Add("content-type", "application/json")
	request.Header.Add("web_session", getUsersRequest.WebSession)
	request.Header.Add("Authorization", "Bearer "+getUsersRequest.AccessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to get user" + err.Error())
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to marshal response" + err.Error())
	}

	if response.StatusCode != 200 {
		return GetUsersResponse{}, errors.New("failed to get user status is not 200" + string(body))
	}

	// parse response
	var getUsersResponse GetUsersResponse
	err = json.Unmarshal(body, &getUsersResponse)
	if err != nil {
		return GetUsersResponse{}, errors.New("failed to unmarshal response" + err.Error())
	}

	if getUsersResponse.Error.Message != "" {
		getUsersResponse.Error.Code = clickErrorMessageWithStatusCode[getUsersResponse.Error.Message]
	}

	return getUsersResponse, nil
}

type GetUsersRequest struct {
	Domain      string
	AccessToken string
	WebSession  string
}

type GetUsersResponse struct {
	Id      interface{} `json:"id"`
	JsonRPC string      `json:"jsonrpc"`
	Result  struct {
		ClientId           int    `json:"client_id"`
		Name               string `json:"name"`
		Surname            string `json:"surname"`
		Patronym           string `json:"patronym"`
		Gender             string `json:"gender"`
		Birthdate          int    `json:"birthdate"`
		IsIdentified       bool   `json:"is_identified"`
		PhoneNumber        string `json:"phone_number"`
		RegionCode         string `json:"region_code"`
		IdentificationDate int    `json:"identification_date"`
	}
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
