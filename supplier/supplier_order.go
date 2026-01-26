package supplier

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	sdk "github.com/asadbekGo/ett-code-sdk"
	"github.com/spf13/cast"
)

type (
	SupplierData struct {
		Guid           string `json:"guid"`
		Type           string `json:"type"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		APIUrl         string `json:"api_url"`
		Token          string `json:"token"`
		TokenExpiresAt string `json:"token_expire_at"`
		AiShortCode    string `json:"ai_short_code"`
		AuthUrl        string `json:"supplier_auth_url"`
		ContactPerson  string `json:"contact_person"`
		Email          string `json:"email"`
		Phone          string `json:"phone"`
	}
	OrderData struct {
		FirstName          string
		LastName           string
		TotalPax           int
		PaxType            []string
		ProductDate        string
		AgentTransactionId string
		AgentOrderItemId   string
	}
	ProductData struct {
		ProductValue      float64
		LocationShortCode float64
		DFCode            string
		AACode            string
		HpCode            string
		ISGCode           string
		TimezoneOffset    string
		DestinationCity   string
	}
	AdditionalData struct {
		EnvironmentId string
		ProdEnvID     string
	}
)

func CreateOrder(supplier SupplierData,
	order OrderData,
	additionalData AdditionalData,
	productData ProductData,
	SendtoETT func(text string),
	resourceMutex *sync.Mutex,
	ettUcodeApi *sdk.ObjectFunction,
	createOrderItemRequest []map[string]interface{},
	orderItemData map[string]interface{},
	hpCodePaxInfo map[string]PaxInfo,
	highPassFastTrackOrders map[string]HighPassCrateOrderRequest,
	index int,
) (couponCode, errorMessage string, errorResponse sdk.ResponseError) {

	var response = sdk.Response{}
	errorResponse = sdk.ResponseError{}

	switch supplier.Type {
	case "ppg":
		var errorNotification string
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		expireTime = expireTime.Add(-time.Hour * 1)

		if !time.Now().UTC().Before(expireTime) {
			var login LoginResponse
			login, errorNotification, err = LoginPPG(LoginRequest{LoginName: supplier.Username, Password: supplier.Password, URL: supplier.APIUrl})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorResponse.TelegramErrorFile = errorNotification
				errorMessage = errorResponse.ErrorMessage
				return
			}

			var updateSupplierRequest = sdk.Request{Data: map[string]interface{}{"guid": supplier.Guid, "token": login.Token, "token_expire_at": login.Expires}}
			_, response, err = ettUcodeApi.UpdateObject(&sdk.Argument{Ctx: context.Background(), TableSlug: "suppliers", Request: updateSupplierRequest, BlockBuilder: true, DisableFaas: true})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
			supplier.Token = login.Token
		}

		visitDate, err := time.Parse(time.DateOnly, order.ProductDate)
		if err != nil {
			errorResponse.StatusCode = 500
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			if additionalData.EnvironmentId == additionalData.ProdEnvID {
				SendtoETT("[Agent API Create Order: Parse Visit Date] [🔴 Down] Request failed with status code 500")
			}
			return
		}

		couponCode, errorNotification, err = GenerateCouponPPG(CouponInput{
			URL:               supplier.APIUrl,
			Token:             supplier.Token,
			FirstName:         order.FirstName,
			LastName:          order.LastName,
			StartDate:         visitDate.AddDate(0, 0, -1).Format(time.DateOnly),
			EndDate:           visitDate.AddDate(0, 0, 1).Format(time.DateOnly),
			AishortCode:       supplier.AiShortCode,
			ProductValue:      productData.ProductValue,
			LocationShortCode: productData.LocationShortCode,
		})
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}
	case "dreamfolks":
		programmIdInt, err := strconv.Atoi(supplier.AiShortCode)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}

		if IsCurrentDate(order.ProductDate, productData.TimezoneOffset) {
			order.ProductDate, err = Add10Minutes(productData.TimezoneOffset)
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
		}

		couponCodeInt, errorNotification, err := GenerateCouponDF(GenerateCouponDFRequest{
			URL:                supplier.APIUrl,
			Username:           supplier.Username,
			Password:           supplier.Password,
			ProgramId:          programmIdInt,
			FirstName:          order.FirstName + order.LastName,
			RequestIdentifier:  order.AgentTransactionId,
			OutletId:           productData.DFCode,
			ValidFrom:          order.ProductDate,
			BookingReferenceNo: order.AgentOrderItemId,
			TotalVisit:         order.TotalPax,
		})
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}

		couponCode = strconv.Itoa(couponCodeInt)
	case "all_airports":
		var errorNotification string
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			errorResponse.StatusCode = 500
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			if additionalData.EnvironmentId == additionalData.ProdEnvID {
				SendtoETT("[Create Order - Every Lounge Token Expire Time Parsing] [🔴 Down] Request failed with status code 500")
			}
			return
		}
		expireTime = expireTime.Add(-time.Minute * 10)

		if !time.Now().UTC().Before(expireTime) {
			var login LoginResponse
			login, errorNotification, err = LoginEveryLounge(LoginRequest{
				LoginName: supplier.Username,
				AuthURL:   supplier.AuthUrl,
				Username:  supplier.Username,
				Password:  supplier.Password,
				URL:       supplier.APIUrl,
			})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorResponse.TelegramErrorFile = errorNotification
				errorMessage = errorResponse.ErrorMessage
				return
			}

			var updateSupplierRequest = sdk.Request{Data: map[string]interface{}{
				"guid":            supplier.Guid,
				"token":           login.Token,
				"token_expire_at": login.Expires,
			}}
			_, response, err = ettUcodeApi.UpdateObject(&sdk.Argument{
				TableSlug:    "suppliers",
				Request:      updateSupplierRequest,
				BlockBuilder: true,
				DisableFaas:  true,
			})
			if err != nil {
				errorResponse.StatusCode = 500
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				if additionalData.EnvironmentId == additionalData.ProdEnvID {
					SendtoETT("[Create Order - Every Lounge Token Update] [🔴 Down] Request failed with status code 500")
				}
				return
			}
			supplier.Token = login.Token
		}

		if len(order.PaxType) <= 0 {
			errorResponse.StatusCode = 500
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			if additionalData.EnvironmentId == additionalData.ProdEnvID {
				SendtoETT("[Create Order - Every Lounge Pax Type] [🔴 Down] Request failed with status code 500")
			}
			return
		}

		createOrderEveryLoungeResponse, errorNotification, err := CreateOrderAllAirports(AAGenerateCouponRequest{
			URL:                 supplier.APIUrl,
			Token:               supplier.Token,
			SupplierAiShortCode: supplier.AiShortCode,
			FirstName:           order.FirstName,
			LastName:            order.LastName,
			PaxType:             order.PaxType[0],
			DestinationCity:     productData.DestinationCity,
			VisitDate:           order.ProductDate,
			AACode:              productData.AACode,
			ContactName:         supplier.ContactPerson,
			ContactEmail:        supplier.Email,
			ContactPhone:        supplier.Phone,
		})
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}

		payResponse, errorNotification, err := PayAllAirports(AAGenerateCouponRequest{
			URL:     supplier.APIUrl,
			Token:   supplier.Token,
			OrderID: cast.ToInt(createOrderEveryLoungeResponse["id"]),
		})
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}
		couponCode = cast.ToString(payResponse["pnr"])

		resourceResponse, errorNotification, err := GetResourceID(AAGenerateCouponRequest{
			URL:    supplier.APIUrl,
			Token:  supplier.Token,
			AACode: productData.AACode,
		})
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}

		var organization = cast.ToStringMap(resourceResponse["organization"])
		AAFlightInfo := fmt.Sprintf(`{"orderId": %d, "organizationId": "%s",  "city": {"id": "%s"}, "type": "Departure", "date": "%s", "number": "-"}`, cast.ToInt(createOrderEveryLoungeResponse["id"]), organization["id"], productData.DestinationCity, order.ProductDate)
		createOrderItemRequest[index]["flight_info"] = AAFlightInfo
	case "highpass":
		var errorNotification string
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		expireTime = expireTime.Add(-time.Hour * 1)

		if !time.Now().UTC().Before(expireTime) {
			var login LoginResponse
			login, errorNotification, err = LoginHighPass(LoginRequest{AiShortCode: supplier.AiShortCode, URL: supplier.APIUrl})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorResponse.TelegramErrorFile = errorNotification
				errorMessage = errorResponse.ErrorMessage
				return
			}

			var updateSupplierRequest = sdk.Request{Data: map[string]interface{}{"guid": supplier.Guid, "token": login.Token, "token_expire_at": login.Expires}}
			_, response, err = ettUcodeApi.UpdateObject(&sdk.Argument{Ctx: context.Background(), TableSlug: "suppliers", Request: updateSupplierRequest, BlockBuilder: true, DisableFaas: true})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
			supplier.Token = login.Token
		}

		var (
			key             = productData.HpCode + "|" + order.ProductDate
			flightInfoStr   = cast.ToString(orderItemData["flight_info"])
			counts          = hpCodePaxInfo[key]
			otherPassengers = strings.Join(counts.Names, ", ")
			flightInfo      = FlightInfo{}
		)
		createOrderItemRequest[index]["highPassKey"] = key

		if len(flightInfoStr) > 0 {
			err = json.Unmarshal([]byte(flightInfoStr), &flightInfo)
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
		}

		if flightInfo.FlightNumber != "" {
			if counts.TerminalType == "arrival" || counts.TerminalType == "transit" {
				order.ProductDate = flightInfo.ArrivalTime
			} else {
				order.ProductDate = flightInfo.DepartureTime
			}
		} else {
			if IsCurrentDate(order.ProductDate, counts.Offset) {
				order.ProductDate, err = Add10Minutes(counts.Offset)
				if err != nil {
					errorResponse.StatusCode = 422
					errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
					errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
					errorMessage = errorResponse.ErrorMessage
					return
				}
			}
		}

		if counts.FlightNumber == "" {
			counts.FlightNumber = "BA396"
		}

		if counts.BaggageCount > 0 {
			counts.Comment += "Passenger has " + cast.ToString(counts.BaggageCount) + " baggage(s); "
		}

		if counts.CompanionsCount > 0 {
			counts.Comment += "Passenger has " + cast.ToString(counts.CompanionsCount) + " companion(s); "
		}

		resourceMutex.Lock()
		if _, ok := highPassFastTrackOrders[key]; !ok {
			fastTrackOrder := HighPassCrateOrderRequest{
				URL:        supplier.APIUrl,
				Token:      supplier.Token,
				PrivateKey: supplier.Password,
				Order: HighPassOrder{
					PublicAPIKey: supplier.AiShortCode,
					Orders: []HighPassOrderItem{
						{
							ServiceID:                              productData.HpCode,
							ServiceDate:                            order.ProductDate,
							FirstName:                              order.FirstName,
							LastName:                               order.LastName,
							AdultCount:                             counts.Adults,
							ChildCount:                             counts.Children,
							FlightNumber:                           counts.FlightNumber,
							Email:                                  supplier.Email,
							Phone:                                  "+998945553322",
							Culture:                                "en-US",
							IsDateTimeOfPassengersArrivalToAirport: true,
							OtherPassengersContactDetails:          otherPassengers,
							FlightRouteData:                        counts.FlightRouteData,
							CarPlateNumber:                         counts.VehicleLicensePlate,
							Comment:                                counts.Comment,
						},
					},
				},
			}

			highPassFastTrackOrders[key] = fastTrackOrder
		}
		resourceMutex.Unlock()
	case "isg":
		var errorNotification string
		var createISGServiceRequest = ISGServiceRequest{
			URL:         supplier.APIUrl,
			AuthKey:     supplier.Password,
			FirstName:   order.FirstName,
			LastName:    order.LastName,
			ProductID:   productData.ISGCode,
			IsTest:      true,
			MaxUseCount: order.TotalPax,
		}

		if additionalData.EnvironmentId == additionalData.ProdEnvID {
			createISGServiceRequest.IsTest = false
		}

		createISGServiceResponse, errorNotification, err := CreateISGService(createISGServiceRequest)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorResponse.TelegramErrorFile = errorNotification
			errorMessage = errorResponse.ErrorMessage
			return
		}
		couponCode = createISGServiceResponse.Data.Code
	}
	return couponCode, errorMessage, errorResponse
}

type (
	// ppg
	LoginRequest struct {
		LoginName   string `json:"loginName"`
		AuthURL     string `json:"authUrl"`
		AiShortCode string `json:"ai_short_code"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		URL         string `json:"url"`
	}
	LoginResponse struct {
		ID          string   `json:"id"`
		Session     string   `json:"session"`
		Token       string   `json:"token"`
		Expires     string   `json:"expires"`
		Permissions []string `json:"permissions"`
	}
	CouponResponse struct {
		Status      int    `json:"status"`
		Tag         string `json:"tag"`
		Description string `json:"description"`
		Result      struct {
			Coupon string `json:"coupon"`
		} `json:"result"`
		Input struct {
			AiShortCode       string `json:"aiShortCode"`
			StartDate         string `json:"startDate"`
			EndDate           string `json:"endDate"`
			FirstName         string `json:"firstName"`
			LastName          string `json:"lastName"`
			LocationShortCode []int  `json:"locationShortCode"`
			Offer             string `json:"offer"`
			Prefix            string `json:"prefix"`
			Remarks           string `json:"remarks"`
			AccessModule      string `json:"access_module"`
			AccessAction      string `json:"access_action"`
		} `json:"input"`
	}

	CouponInput struct {
		URL               string
		Token             string
		FirstName         string
		LastName          string
		StartDate         string
		EndDate           string
		AishortCode       string
		ProductValue      float64
		LocationShortCode float64
	}
	GenerateRequest struct {
		AIShortCode       string    `json:"aiShortCode"`
		StartDate         string    `json:"startDate"`
		EndDate           string    `json:"endDate"`
		FirstName         string    `json:"firstName"`
		LastName          string    `json:"lastName"`
		LocationShortCode []float64 `json:"locationShortCode"`
		Offer             string    `json:"offer"`
		Prefix            string    `json:"prefix"`
	}
	// Dreamfolks
	GenerateCouponDFRequest struct {
		URL                string
		Username           string
		Password           string
		ProgramId          int
		ServiceId          int
		FirstName          string
		RequestIdentifier  string
		OutletId           string
		Email              string
		ValidFrom          string
		BookingReferenceNo string
		TotalVisit         int
	}
	GenerateCouponDFAPIRequest struct {
		ProgramId          int    `json:"program_id"`
		ServiceId          string `json:"service_id"`
		FirstName          string `json:"first_name"`
		RequestIdentifier  string `json:"request_identifier"`
		OutletId           string `json:"outlet_id"`
		Email              string `json:"email"`
		ValidFrom          string `json:"valid_from"`
		BookingReferenceNo string `json:"booking_reference_no"`
		TotalVisit         int    `json:"total_visit"`
	}
	DFGeneratecouponResponse struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Code    int    `json:"code"`
		Data    struct {
			VoucherCode       int    `json:"voucher_code"`
			EcertURL          string `json:"ecert_url"`
			RequestIdentifier string `json:"request_identifier"`
			BillingPriceGroup string `json:"billing_price_group"`
		} `json:"data"`
	}
	// Every Lounge
	AALoginResponse struct {
		AccessToken string `json:"access_token"`
		Expires     int    `json:"expires_in"`
	}
	AAGenerateCouponRequest struct {
		URL                 string  `json:"url"`
		Token               string  `json:"token"`
		SupplierAiShortCode string  `json:"SupplierAiShortCode"`
		FirstName           string  `json:"firstName"`
		LastName            string  `json:"lastName"`
		PaxType             string  `json:"paxType"`
		LocationShortCode   float64 `json:"locationShortCode"`
		DestinationCity     string  `json:"destinationCity"`
		VisitDate           string  `json:"visitDate"`
		AACode              string  `json:"aaCode"`
		ContactName         string  `json:"contactName"`
		ContactEmail        string  `json:"contactEmail"`
		ContactPhone        string  `json:"contactPhone"`
		OrderID             int     `json:"orderId"`
	}
	AAGenerateRequest struct {
		Contract     AAContract    `json:"contract"`
		Type         string        `json:"type"`
		Passengers   []AAPassenger `json:"passengers"`
		Resources    []AAResource  `json:"resources"`
		ContactName  string        `json:"contactName"`
		ContactEmail string        `json:"contactEmail"`
		ContactPhone string        `json:"contactPhone"`
	}
	AAContract struct {
		ID string `json:"id"`
	}
	AAPassenger struct {
		GivenName   string `json:"givenName"`
		FamilyName  string `json:"familyName"`
		MiddleName  string `json:"middleName"`
		DateOfBirth string `json:"dateOfBirth"`
	}
	AAResource struct {
		Resources AAResourceObject `json:"resource"`
		Flights   []AAFlight       `json:"flights"`
	}
	AAResourceObject struct {
		ID string `json:"id"`
	}
	AACity struct {
		ID string `json:"id"`
	}
	AAFlight struct {
		City   AACity `json:"city"`
		Type   string `json:"type"`
		Date   string `json:"date"`
		Number string `json:"number"`
	}
	AAFlightInfo struct {
		City struct {
			ID string `json:"id"`
		} `json:"city"`
		Type   string `json:"type"`
		Date   string `json:"date"`
		Number string `json:"number"`
	}
	// HighPass
	HighPassLoginResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	HighPassCrateOrderRequest struct {
		URL        string
		Token      string
		PrivateKey string
		Coupon     string
		Order      HighPassOrder
	}

	HighPassOrder struct {
		Orders       []HighPassOrderItem `json:"orders"`
		PublicAPIKey string              `json:"publicApiKey"`
	}

	HighPassOrderItem struct {
		ServiceID                              string `json:"serviceId"`
		FlightNumber                           string `json:"flightNumber"`
		ServiceDate                            string `json:"serviceDate"` // or time.Time if you want to parse it
		AdultCount                             int    `json:"adultCount"`
		ChildCount                             int    `json:"childCount"`
		InfantCount                            int    `json:"infantCount"`
		FirstName                              string `json:"firstName"`
		LastName                               string `json:"lastName"`
		Email                                  string `json:"email"`
		Phone                                  string `json:"phone"`
		Culture                                string `json:"culture"`
		IsDateTimeOfPassengersArrivalToAirport bool   `json:"isDateTimeOfPassengersArrivalToAirport"`
		OtherPassengersContactDetails          string `json:"otherPassengersContactDetails,omitempty"`
		Comment                                string `json:"comment,omitempty"`
		CarPlateNumber                         string `json:"carPlateNumber,omitempty"`
		FlightRouteData                        string `json:"flightRouteData,omitempty"`
	}
	PaxInfo struct {
		Adults                   int
		Children                 int
		Names                    []string
		FlightNumber             string
		TerminalType             string
		Offset                   string
		HasAnimal                bool
		AdditionalPhoneNumber    string
		DriverPhoneNumber        string
		VehicleLicensePlate      string
		PreferredServiceLanguage string
		Comment                  string
		FlightRouteData          string
		CompanionsCount          int
		BaggageCount             int
		NeedStroller             bool
		NeedsWheelchair          bool
		FlightRoute              string
		Phone                    string
		Category                 int
	}
	HighPassCouponData struct {
		CouponCode string `json:"coupon_code"`
		QrData     string `json:"qr_data"`
		OrderId    string `json:"order_id"`
	}

	HighPassHashedOrderRuqust struct {
		Data      string `json:"data"`
		Signature string `json:"signature"`
	}
	OrderResponse struct {
		Orders []Order `json:"orders"`
	}

	Order struct {
		PassengerName    string   `json:"passengerName"`
		HighPassOrderID  string   `json:"highPassOrderId"`
		BookingCode      string   `json:"bookingCode"`
		OrderNumber      int      `json:"orderNumber"`
		AirportIATACode  string   `json:"airportIataCode"`
		ServiceID        string   `json:"serviceId"`
		ServiceName      string   `json:"serviceName"`
		ServiceDateLocal string   `json:"serviceDateLocal"`
		Price            float64  `json:"price"`
		QRData           []string `json:"qrData"`
		ErrorMessage     string   `json:"errorMessage"`
	}
	// isg
	ISGServiceRequest struct {
		URL         string
		AuthKey     string
		FirstName   string
		LastName    string
		ProductID   string
		IsTest      bool
		MaxUseCount int
	}

	ISGServiceResponse struct {
		Error        bool   `json:"Error"`
		ErrorMessage string `json:"ErrorMessage"`
		ErrorCode    int    `json:"ErrorCode"`
		Data         struct {
			Code    string `json:"Code"`
			PassUrl string `json:"PassUrl"`
		} `json:"Data"`
	}
	// additional
	FlightInfo struct {
		FlightNumber                   string  `json:"flightNumber"`
		DepartureTime                  string  `json:"departureTime"`
		DepartureAirportIATACode       string  `json:"departureAirportIATACode"`
		DepartureTerminal              string  `json:"departureTerminal"`
		ArrivalAirportIATACode         string  `json:"arrivalAirportIATACode"`
		ArrivalTerminal                string  `json:"arrivalTerminal"`
		ArrivalTime                    string  `json:"arrivalTime"`
		Airline                        Airline `json:"airline"`
		DepartureCountry               string  `json:"departureCountry"`
		ArrivalCountry                 string  `json:"arrivalCountry"`
		ArrivalAirportTimezoneOffset   float64 `json:"arrivalAirportTimezoneOffset"`
		DepartureAirportTimezoneOffset float64 `json:"departureAirportTimezoneOffset"`
		DepartureTerminalId            string  `json:"departureTerminalId"`
		IsStaticDepartureAirportCode   bool    `json:"IsStaticDepartureAirportCode"`
		VisitDate                      string  `json:"visitDate"`
	}

	Airline struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
)

func LoginPPG(req LoginRequest) (LoginResponse, string, error) {
	var generalErrorMessage string
	jsonData, err := json.Marshal(LoginRequest{LoginName: req.LoginName, Password: req.Password})
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to marshal request: " + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	httpReq, err := http.NewRequest("POST", req.URL+"/api/fe/v1/user/login", bytes.NewBuffer(jsonData))
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create login request: " + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to send login request:" + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body: " + string(jsonData))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to read login response body:" + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body: " + string(jsonData))
	}

	if resp.StatusCode != http.StatusOK {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed. Invalid status code in login:", resp.StatusCode) + " Response body: " + string(body)
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	var loginResponse LoginResponse
	err = json.Unmarshal(body, &loginResponse)
	if err != nil {
		generalErrorMessage = "Supplier login API request failed. Error decoding JSON response: " + err.Error() + " Response body: " + string(body)
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	return loginResponse, "", err
}

func GenerateCouponPPG(couponData CouponInput) (string, string, error) {
	var generalErrorMessage string
	jsonData, err := json.Marshal(GenerateRequest{
		AIShortCode:       couponData.AishortCode,
		StartDate:         couponData.StartDate,
		EndDate:           couponData.EndDate,
		FirstName:         couponData.FirstName,
		LastName:          couponData.LastName,
		LocationShortCode: []float64{couponData.LocationShortCode},
		Offer:             fmt.Sprintf("PPGETT%vHR", couponData.ProductValue),
		Prefix:            "ETT",
	})
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to marshal request: " + err.Error()
		return "", generalErrorMessage, errors.New(generalErrorMessage)
	}

	req, err := http.NewRequest("POST", couponData.URL+"/api/master/v1/coupon/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create request: " + err.Error()
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	req.Header.Set("Authorization", "Bearer "+couponData.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to send coupon generation request: " + err.Error()
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to read coupon generation response body: " + err.Error()
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	if resp.StatusCode != http.StatusOK {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed. Invalid status code in coupon generation:", resp.StatusCode) + " Response body: " + string(body)
		return "", generalErrorMessage, errors.New(generalErrorMessage + "body:" + string(body))
	}

	var coupon CouponResponse
	if err := json.Unmarshal(body, &coupon); err != nil {
		generalErrorMessage = "Supplier API request failed. Error decoding JSON response:" + err.Error() + " Response body:" + string(body)
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	if coupon.Description == "Could not find the offer in AI" {
		generalErrorMessage = "Supplier API request failed. Product value is not found"
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	if coupon.Status != 1 {
		generalErrorMessage = "Supplier API request failed. Invalid Coupon status:" + strconv.Itoa(coupon.Status) + " Response body:" + string(body)
		return "", generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	return strings.ReplaceAll(coupon.Result.Coupon, "@ppg", ""), "", nil
}

func GenerateCouponDF(couponData GenerateCouponDFRequest) (int, string, error) {
	var generalErrorMessage string
	jsonData, err := json.Marshal(GenerateCouponDFAPIRequest{
		ProgramId:          couponData.ProgramId,
		ServiceId:          "21",
		FirstName:          couponData.FirstName,
		RequestIdentifier:  couponData.RequestIdentifier,
		OutletId:           couponData.OutletId,
		Email:              couponData.Email,
		ValidFrom:          couponData.ValidFrom,
		BookingReferenceNo: couponData.BookingReferenceNo,
		TotalVisit:         couponData.TotalVisit,
	})
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to marshal request: " + err.Error()
		return 0, generalErrorMessage, errors.New(generalErrorMessage)
	}

	req, err := http.NewRequest("POST", couponData.URL+"/api/get-voucher-outlet", bytes.NewBuffer(jsonData))
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create request: " + err.Error()
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	req.Header.Set("key", couponData.Username)
	req.Header.Set("secret", couponData.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 100 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to send request:" + err.Error()
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body: " + string(jsonData))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to read response body:" + err.Error()
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body: " + string(jsonData))
	}

	if resp.StatusCode != http.StatusOK {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed. Invalid status code:", resp.StatusCode) + " Response body: " + string(body)
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body: " + string(jsonData))
	}

	var coupon DFGeneratecouponResponse
	if err := json.Unmarshal(body, &coupon); err != nil {
		generalErrorMessage = "Supplier API request failed. Error decoding JSON response:" + err.Error() + " Response body:" + string(body)
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	if !coupon.Status {
		generalErrorMessage = "Supplier API request failed. Received false coupon status. Response body:" + string(body)
		return 0, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	return coupon.Data.VoucherCode, "", nil
}

func LoginEveryLounge(req LoginRequest) (LoginResponse, string, error) {
	var generalErrorMessage string
	url := req.AuthURL + "/connect/token"

	payload := strings.NewReader(
		"grant_type=client_credentials" +
			"&client_id=" + req.Username +
			"&client_secret=" + req.Password +
			"&scope=ResourceServerApi")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	request, err := http.NewRequest("POST", url, payload)
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create login request: " + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(request)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to send login request: " + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		generalErrorMessage = "Supplier API request failed. Invalid status code in login: " + strconv.Itoa(res.StatusCode)
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to read login response body: " + err.Error()
		return LoginResponse{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	var loginResponse AALoginResponse
	err = json.Unmarshal(body, &loginResponse)

	return LoginResponse{
		Token:   loginResponse.AccessToken,
		Expires: time.Now().Add(time.Duration(loginResponse.Expires) * time.Second).Format(time.RFC3339),
	}, "", err
}

func CreateOrderAllAirports(couponData AAGenerateCouponRequest) (map[string]interface{}, string, error) {
	var generalErrorMessage string
	paxDateOfBirth := time.Now().AddDate(-20, 0, 0).Format("2006-01-02T15:04:05")

	jsonData, err := json.Marshal(AAGenerateRequest{
		Contract: AAContract{
			ID: couponData.SupplierAiShortCode,
		},
		Type: "Standard",
		Passengers: []AAPassenger{
			{
				GivenName:   couponData.FirstName,
				FamilyName:  couponData.LastName,
				DateOfBirth: paxDateOfBirth,
			},
		},
		Resources: []AAResource{
			{
				Resources: AAResourceObject{
					ID: couponData.AACode,
				},
				Flights: []AAFlight{
					{
						City: AACity{
							ID: couponData.DestinationCity,
						},
						Type:   "Departure",
						Date:   couponData.VisitDate,
						Number: "-",
					},
				},
			},
		},
		ContactName:  couponData.ContactName,
		ContactEmail: couponData.ContactEmail,
		ContactPhone: couponData.ContactPhone,
	})
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to Marshal Creater Order request body: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage)
	}

	req, err := http.NewRequest(http.MethodPost, couponData.URL+"/api/v0/orders", bytes.NewBuffer(jsonData))
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create Creater Order request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	req.Header.Set("Authorization", "Bearer "+couponData.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to do Creater Order request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to read Creater Order response: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	if resp.StatusCode >= 400 {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed with status code:", resp.StatusCode, "response body:", string(body))
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed. Failed to Unmarshal response body, error: ", err.Error())
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(jsonData))
	}

	return response, "", nil
}

func PayAllAirports(request AAGenerateCouponRequest) (map[string]interface{}, string, error) {
	var generalErrorMessage string
	if request.OrderID == 0 {
		generalErrorMessage = "Supplier API request failed, ResourceID and OrderID are required"
		return nil, generalErrorMessage, errors.New(generalErrorMessage)
	}

	var payload = `{
		"mode": "Offline",
		"type": "Card"
	}`

	var url = fmt.Sprintf("%s/api/v0/orders/%d/pay", request.URL, request.OrderID)
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(payload))
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create Pay Order request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%d", request.OrderID))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+request.Token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to do Pay Order request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%d", request.OrderID))
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to read Pay Order response: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%d", request.OrderID))
	}

	if resp.StatusCode >= 400 {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed with status code:", resp.StatusCode, "response body:", string(body))
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%d", request.OrderID))
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to Unmarshal response body, error: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%d", request.OrderID))
	}

	return response, "", nil
}

func GetResourceID(request AAGenerateCouponRequest) (map[string]interface{}, string, error) {
	var generalErrorMessage string
	if request.AACode == "" {
		generalErrorMessage = "Supplier API request failed, ResourceID and AA Code are required"
		return nil, generalErrorMessage, errors.New(generalErrorMessage)
	}

	var url = fmt.Sprintf("%s/api/v0/resources/%s", request.URL, request.AACode)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create Get ResourceID request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", request.AACode))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+request.Token)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to do Get ResourceID request: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", request.AACode))
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to read Get ResourceID response: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", request.AACode))
	}

	if resp.StatusCode >= 400 {
		generalErrorMessage = fmt.Sprintln("Supplier API request failed with status code:", resp.StatusCode, "response body:", string(body))
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", request.AACode))
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to Unmarshal response body, error: " + err.Error()
		return nil, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", request.AACode))
	}

	return response, "", nil
}

func LoginHighPass(req LoginRequest) (LoginResponse, string, error) {
	var generalErrorMessage string
	url := req.URL + "/api/v1/token"

	payload := strings.NewReader(
		"grant_type=client_credentials" +
			"&client_type=ThirdParty" +
			"&api_key=" + req.AiShortCode)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	request, err := http.NewRequest("POST", url, payload)

	if err != nil {
		generalErrorMessage = "Internal Server Error. Failed to create Login request: " + err.Error()
		return LoginResponse{}, "", errors.New(generalErrorMessage + req.AiShortCode)
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(request)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to send Login request: " + err.Error()
		return LoginResponse{}, "", errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", req.AiShortCode))
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		generalErrorMessage = "Supplier API request failed. Login request failed with status code: " + res.Status
		return LoginResponse{}, "", errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", req.AiShortCode))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed. Failed to read Login response body: " + err.Error()
		return LoginResponse{}, "", errors.New(generalErrorMessage + " Request body:" + fmt.Sprintf("%s", req.AiShortCode))
	}

	var loginResponse HighPassLoginResponse
	err = json.Unmarshal(body, &loginResponse)

	return LoginResponse{
		Token:   loginResponse.AccessToken,
		Expires: time.Now().Add(time.Duration(loginResponse.ExpiresIn) * time.Second).Format(time.RFC3339),
	}, "", err
}

func CreateHighPass(request HighPassCrateOrderRequest) (HighPassCouponData, string, error) {
	var generalErrorMessage string
	jsonOrderData, err := json.Marshal(HighPassOrder{
		PublicAPIKey: request.Order.PublicAPIKey,
		Orders:       request.Order.Orders,
	},
	)
	if err != nil {
		generalErrorMessage = "Internal server error, failed to marshal HighPass Create Order request body: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	jsonDataBase64 := base64.StdEncoding.EncodeToString(jsonOrderData)

	signatureSha1 := sha1ToBase64(request.PrivateKey + jsonDataBase64 + request.PrivateKey)

	jsonData, err := json.Marshal(HighPassHashedOrderRuqust{
		Data:      jsonDataBase64,
		Signature: signatureSha1,
	})
	if err != nil {
		generalErrorMessage = "Internal server error, failed to marshal HighPass hashed Create Order request body: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	req, err := http.NewRequest("POST", request.URL+"/api/v1/orders", bytes.NewBuffer(jsonData))
	if err != nil {
		generalErrorMessage = "Internal server error, failed to create HighPass Create Order request: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	req.Header.Set("Authorization", "Bearer "+request.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to do HighPass Create Order request: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to read HighPass Create Order response: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage)
	}

	if resp.StatusCode != http.StatusOK {
		generalErrorMessage = "Supplier API request failed, HighPass Create Order request failed with status code: " + fmt.Sprint(resp.StatusCode)
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	var order OrderResponse
	if err := json.Unmarshal(body, &order); err != nil {
		generalErrorMessage = "Supplier API request failed, failed to unmarshal HighPass Create Order response: " + err.Error()
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	if len(order.Orders) <= 0 {
		generalErrorMessage = "Supplier API request failed, no orders found in response body"
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	if order.Orders[0].ErrorMessage != "" {
		generalErrorMessage = "Supplier API request failed, HighPass Create Order request failed with error message: " + order.Orders[0].ErrorMessage
		return HighPassCouponData{}, generalErrorMessage, errors.New(generalErrorMessage + " Request body:" + string(body))
	}

	response := HighPassCouponData{
		CouponCode: order.Orders[0].BookingCode,
		OrderId:    order.Orders[0].HighPassOrderID,
	}

	if len(order.Orders[0].QRData) > 0 {
		response.QrData = order.Orders[0].QRData[0]
	}

	return response, "", nil
}

func sha1ToBase64(data string) string {
	hash := sha1.Sum([]byte(data))                    // SHA1 hash (returns [20]byte)
	return base64.StdEncoding.EncodeToString(hash[:]) // Convert to base64 string
}

func CreateISGService(reqData ISGServiceRequest) (ISGServiceResponse, string, error) {
	var generalErrorMessage string
	params := url.Values{}
	params.Set("authKey", reqData.AuthKey)
	params.Set("firstname", reqData.FirstName)
	params.Set("lastname", reqData.LastName)
	params.Set("productid", reqData.ProductID)
	params.Set("isTest", fmt.Sprintf("%t", reqData.IsTest))
	if reqData.MaxUseCount > 0 {
		params.Set("maxusecount", fmt.Sprintf("%d", reqData.MaxUseCount))
	}

	var fullURL = reqData.URL + "/premiumservices/create?" + params.Encode()
	req, err := http.NewRequest(http.MethodPost, fullURL, nil)
	if err != nil {
		generalErrorMessage = "Internal Server Error, failed to create ISG Service request: " + err.Error()
		return ISGServiceResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " URL:" + fullURL)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to do ISG Service request: " + err.Error()
		return ISGServiceResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " URL:" + fullURL)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		generalErrorMessage = "Supplier API request failed, failed to read ISG Service response: " + err.Error()
		return ISGServiceResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " URL:" + fullURL)
	}

	if resp.StatusCode != http.StatusOK {
		generalErrorMessage = "Supplier API request failed, unexpected status code: " + fmt.Sprint(resp.StatusCode)
		return ISGServiceResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " URL:" + fullURL)
	}

	var result ISGServiceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		generalErrorMessage = "Supplier API request failed, failed to unmarshal ISG Service response: " + err.Error()
		return ISGServiceResponse{}, generalErrorMessage, errors.New(generalErrorMessage + " URL:" + fullURL)
	}

	if result.Error {
		generalErrorMessage = "Supplier API request failed, ISG Service Error: " + result.ErrorMessage
		return result, generalErrorMessage, errors.New(generalErrorMessage + " URL: " + fullURL + " body: " + string(body))
	}

	return result, "", nil
}

func IsCurrentDate(serviceDate, timezoneOffset string) bool {

	date, err := time.Parse(time.DateOnly, serviceDate)
	if err != nil {
		return false
	}

	timeZoneOffsetStringHour := strings.Split(timezoneOffset, ":")[0] + "h"
	timeZoneOffsetStringMinute := strings.Split(timezoneOffset, ":")[1] + "m"

	timeZoneOffset, err := time.ParseDuration(timeZoneOffsetStringHour + timeZoneOffsetStringMinute)
	if err != nil {
		return false
	}

	return time.Now().Add(timeZoneOffset).Format(time.DateOnly) == date.Format(time.DateOnly)
}

func Add10Minutes(timezoneOffset string) (string, error) {

	timeZoneOffsetStringHour := strings.Split(timezoneOffset, ":")[0] + "h"
	timeZoneOffsetStringMinute := strings.Split(timezoneOffset, ":")[1] + "m"

	timeZoneOffset, err := time.ParseDuration(timeZoneOffsetStringHour + timeZoneOffsetStringMinute)
	if err != nil {
		return "", err
	}

	currentTime := time.Now().Add(timeZoneOffset).Add(10 * time.Minute).Format(time.DateTime)

	return currentTime, nil
}
