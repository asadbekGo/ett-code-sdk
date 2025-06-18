package ets_hub

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
)

var etsHubErrorCodeWithSdkCode = map[int]int{
	101: 401,
	102: 400,
	103: 403,
	201: 400,
	202: 404,
	301: 404,
	302: 404,
	303: 404,
}

type ETSHubRequest struct {
	Token       string                `json:"token"`
	PayRegister ETSHubPayRegisterBody `json:"payRegister"`
	CheckPay    CheckPayRequest       `json:"checkStatus"`
	EttUcodeApi *sdk.ObjectFunction   `json:"ettUcodeApi"`
	BaseUrl     string                `json:"baseUrl"`
}

type ETSHubPayRegisterBody struct {
	PspID       int      `json:"psp_id"`
	Amount      int      `json:"amount"`
	Lifetime    int      `json:"lifetime"`
	Currency    string   `json:"currency"`
	SuccessURL  string   `json:"success_url"`
	FailURL     string   `json:"fail_url"`
	CallbackURL string   `json:"callback_url"`
	Details     *Details `json:"details,omitempty"`
	Signature   string   `json:"signature,omitempty"`
}

type Details struct {
	OrderID  string `json:"order_id,omitempty"`
	Customer string `json:"customer,omitempty"`
	Email    string `json:"email,omitempty"`
}

type ETSHubPayRegisterResponse struct {
	ErrorCode    int                    `json:"error_code"`
	ErrorMessage string                 `json:"error_message"`
	OrderID      string                 `json:"order_id"`
	Status       string                 `json:"status"`
	Amount       int                    `json:"amount"`
	Currency     string                 `json:"currency"`
	Details      map[string]interface{} `json:"details"`
	RedirectURL  string                 `json:"redirect_url"`
}

type CheckPayRequest struct {
	OrderId   string `json:"order_id"`
	Amount    int    `json:"amount"`
	Currency  string `json:"currency"`
	Signature string `json:"signature,omitempty"`
}

func GetSignatureHash(secretKey string, request ETSHubPayRegisterBody) (signature string, errorResponse sdk.ResponseError) {

	body, err := json.Marshal(request)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = err.Error()
		return signature, errorResponse
	}

	var bodyString = string(body)
	bodyString = strings.ReplaceAll(bodyString, `/`, `\/`)
	signature = SimpleEncoding(secretKey, bodyString)

	return signature, errorResponse
}

func PayRegister(req ETSHubRequest) (payRegister ETSHubPayRegisterResponse, errorResponse sdk.ResponseError) {
	var headers = map[string]interface{}{
		"payment-hub-token": req.Token,
		"content-type":      "application/json",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	body, err := sdk.DoRequest(ctx, req.BaseUrl+"/hub/pay/register", http.MethodPost, req.PayRegister, "", headers)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error())
		return payRegister, errorResponse
	}

	var response ETSHubPayRegisterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error() + ", body: " + string(body))
		return payRegister, errorResponse
	}

	if response.ErrorCode != 0 {
		errorResponse.StatusCode = etsHubErrorCodeWithSdkCode[response.ErrorCode]
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(fmt.Sprintf("error_code: %d, error_message: %s", response.ErrorCode, response.ErrorMessage))
		return payRegister, errorResponse
	}

	return response, errorResponse
}

func PayCheckStatus(req ETSHubRequest) (payRegister ETSHubPayRegisterResponse, errorResponse sdk.ResponseError) {
	var headers = map[string]interface{}{
		"payment-hub-token": req.Token,
		"content-type":      "application/json",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	body, err := sdk.DoRequest(ctx, req.BaseUrl+"/hub/pay/status", http.MethodPost, req.CheckPay, "", headers)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error())
		return payRegister, errorResponse
	}

	var response ETSHubPayRegisterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error() + ", body: " + string(body))
		return payRegister, errorResponse
	}

	if response.ErrorCode != 0 {
		errorResponse.StatusCode = etsHubErrorCodeWithSdkCode[response.ErrorCode]
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(fmt.Sprintf("error_code: %d, error_message: %s", response.ErrorCode, response.ErrorMessage))
		return payRegister, errorResponse
	}

	return response, errorResponse
}

func PayConfirm(req ETSHubRequest) (payRegister ETSHubPayRegisterResponse, errorResponse sdk.ResponseError) {
	var headers = map[string]interface{}{
		"payment-hub-token": req.Token,
		"content-type":      "application/json",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	body, err := sdk.DoRequest(ctx, req.BaseUrl+"/hub/pay/confirm", http.MethodPost, req.CheckPay, "", headers)
	if err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error())
		return payRegister, errorResponse
	}

	var response ETSHubPayRegisterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		errorResponse.StatusCode = 500
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(err.Error() + ", body: " + string(body))
		return payRegister, errorResponse
	}

	if response.ErrorCode != 0 && response.ErrorCode != 203 {
		errorResponse.StatusCode = etsHubErrorCodeWithSdkCode[response.ErrorCode]
		errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
		errorResponse.ErrorMessage = req.EttUcodeApi.Logger.ErrorLog.Sprint(fmt.Sprintf("error_code: %d, error_message: %s", response.ErrorCode, response.ErrorMessage))
		return payRegister, errorResponse
	}

	return response, errorResponse
}

func SimpleEncoding(secretKey, jsonData string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(jsonData))
	hmacValue := h.Sum(nil)
	signature := hex.EncodeToString(hmacValue)
	return signature
}
