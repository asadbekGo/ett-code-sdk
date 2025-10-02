package supplier

import (
	"bytes"
	"context"
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
		firstName          string
		lastName           string
		totalPax           int
		paxType            []string
		productDate        string
		agentTransactionId string
		agentOrderItemId   string
	}
	ProductData struct {
		productValue      float64
		locationShortCode float64
		dfCode            string
		aaCode            string
		hpCode            string
		timezoneOffset    string
		destinationCity   string
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
) (couponCode, errorMessage string) {
	var (
		errorResponse = sdk.ResponseError{}
		response      = sdk.Response{}
	)
	switch supplier.Type {
	case "ppg":
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		expireTime = expireTime.Add(-time.Hour * 1)

		if !time.Now().UTC().Before(expireTime) {
			var login LoginResponse
			login, err = LoginPPG(LoginRequest{LoginName: supplier.Username, Password: supplier.Password, URL: supplier.APIUrl})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
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

		visitDate, err := time.Parse(time.DateOnly, order.productDate)
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 500
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			if additionalData.EnvironmentId == additionalData.ProdEnvID {
				SendtoETT("[Agent API Create Order: Parse Visit Date] [🔴 Down] Request failed with status code 500")
			}
			return
		}

		couponCode, err = GenerateCouponPPG(CouponInput{
			URL:               supplier.APIUrl,
			Token:             supplier.Token,
			FirstName:         order.firstName,
			LastName:          order.lastName,
			StartDate:         visitDate.AddDate(0, 0, -1).Format(time.DateOnly),
			EndDate:           visitDate.AddDate(0, 0, 1).Format(time.DateOnly),
			AishortCode:       supplier.AiShortCode,
			ProductValue:      productData.productValue,
			LocationShortCode: productData.locationShortCode,
		})
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
	case "dreamfolks":
		programmIdInt, err := strconv.Atoi(supplier.AiShortCode)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}

		if IsCurrentDate(order.productDate, productData.timezoneOffset) {
			order.productDate, err = Add10Minutes(productData.timezoneOffset)
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
		}

		couponCodeInt, err := GenerateCouponDF(GenerateCouponDFRequest{
			URL:                supplier.APIUrl,
			Username:           supplier.Username,
			Password:           supplier.Password,
			ProgramId:          programmIdInt,
			FirstName:          order.firstName + order.lastName,
			RequestIdentifier:  order.agentTransactionId,
			OutletId:           productData.dfCode,
			ValidFrom:          order.productDate,
			BookingReferenceNo: order.agentOrderItemId,
			TotalVisit:         order.totalPax,
		})
		if err != nil {
			// ettUcodeApi.SendTelegram("GenerateCouponDF err:" + err.Error())
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}

		couponCode = strconv.Itoa(couponCodeInt)
	case "all_airports":
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 500
			errorResponse.Description = response.Data["description"]
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
			login, err = LoginEveryLounge(LoginRequest{
				LoginName: supplier.Username,
				AuthURL:   supplier.AuthUrl,
				Username:  supplier.Username,
				Password:  supplier.Password,
				URL:       supplier.APIUrl,
			})
			if err != nil {
				resourceMutex.Lock()
				defer resourceMutex.Unlock()
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
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
				resourceMutex.Lock()
				defer resourceMutex.Unlock()
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

		if len(order.paxType) <= 0 {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 500
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			if additionalData.EnvironmentId == additionalData.ProdEnvID {
				SendtoETT("[Create Order - Every Lounge Pax Type] [🔴 Down] Request failed with status code 500")
			}
			return
		}

		createOrderEveryLoungeResponse, err := CreateOrderAllAirports(AAGenerateCouponRequest{
			URL:                 supplier.APIUrl,
			Token:               supplier.Token,
			SupplierAiShortCode: supplier.AiShortCode,
			FirstName:           order.firstName,
			LastName:            order.lastName,
			PaxType:             order.paxType[0],
			DestinationCity:     productData.destinationCity,
			VisitDate:           order.productDate,
			AACode:              productData.aaCode,
			ContactName:         supplier.ContactPerson,
			ContactEmail:        supplier.Email,
			ContactPhone:        supplier.Phone,
		})
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}

		payResponse, err := PayAllAirports(AAGenerateCouponRequest{
			URL:     supplier.APIUrl,
			Token:   supplier.Token,
			OrderID: cast.ToInt(createOrderEveryLoungeResponse["id"]),
		})
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		couponCode = cast.ToString(payResponse["pnr"])

		resourceResponse, err := GetResourceID(AAGenerateCouponRequest{
			URL:    supplier.APIUrl,
			Token:  supplier.Token,
			AACode: productData.aaCode,
		})
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}

		var organization = cast.ToStringMap(resourceResponse["organization"])
		AAFlightInfo := fmt.Sprintf(`{"orderId": %d, "organizationId": "%s",  "city": {"id": "%s"}, "type": "Departure", "date": "%s", "number": "-"}`, cast.ToInt(createOrderEveryLoungeResponse["id"]), organization["id"], productData.destinationCity, order.productDate)
		createOrderItemRequest[index]["flight_info"] = AAFlightInfo
	case "highpass":
		expireTime, err := time.Parse(time.RFC3339, supplier.TokenExpiresAt)
		if err != nil {
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		expireTime = expireTime.Add(-time.Hour * 1)

		if !time.Now().UTC().Before(expireTime) {
			var login LoginResponse
			login, err = LoginHighPass(LoginRequest{AiShortCode: supplier.AiShortCode, URL: supplier.APIUrl})
			if err != nil {
				errorResponse.StatusCode = 422
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
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
			key             = productData.hpCode + "|" + order.productDate
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
				errorResponse.Description = response.Data["description"]
				errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
				errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
				errorMessage = errorResponse.ErrorMessage
				return
			}
		}

		if flightInfo.FlightNumber != "" {
			if counts.TerminalType == "arrival" || counts.TerminalType == "transit" {
				order.productDate = flightInfo.ArrivalTime
			} else {
				order.productDate = flightInfo.DepartureTime
			}
		} else {
			if IsCurrentDate(order.productDate, counts.Offset) {
				order.productDate, err = Add10Minutes(counts.Offset)
				if err != nil {
					errorResponse.StatusCode = 422
					errorResponse.Description = response.Data["description"]
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
							ServiceID:                              productData.hpCode,
							ServiceDate:                            order.productDate,
							FirstName:                              order.firstName,
							LastName:                               order.lastName,
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
		var createISGServiceRequest = ISGServiceRequest{
			URL:         supplier.APIUrl,
			AuthKey:     supplier.Password,
			FirstName:   order.firstName,
			LastName:    order.lastName,
			ProductID:   productData.hpCode,
			IsTest:      true,
			MaxUseCount: order.totalPax,
		}

		if additionalData.EnvironmentId == additionalData.ProdEnvID {
			createISGServiceRequest.IsTest = false
		}

		createISGServiceResponse, err := CreateISGService(createISGServiceRequest)
		if err != nil {
			resourceMutex.Lock()
			defer resourceMutex.Unlock()
			errorResponse.StatusCode = 422
			errorResponse.Description = response.Data["description"]
			errorResponse.ClientErrorMessage = sdk.ErrorCodeWithMessage[errorResponse.StatusCode]
			errorResponse.ErrorMessage = ettUcodeApi.Logger.ErrorLog.Sprint(err.Error())
			errorMessage = errorResponse.ErrorMessage
			return
		}
		couponCode = createISGServiceResponse.Data.Code
	}
	return couponCode, errorMessage
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

func LoginPPG(req LoginRequest) (LoginResponse, error) {
	jsonData, err := json.Marshal(LoginRequest{LoginName: req.LoginName, Password: req.Password})
	if err != nil {
		return LoginResponse{}, err
	}

	client := &http.Client{
		Timeout: 40 * time.Second,
	}

	httpReq, err := http.NewRequest("POST", req.URL+"/api/fe/v1/user/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return LoginResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return LoginResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LoginResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return LoginResponse{}, errors.New(fmt.Sprintln("API request failed with status code:", resp.StatusCode, "body:", string(body)))
	}

	var loginResponse LoginResponse
	err = json.Unmarshal(body, &loginResponse)
	if err != nil {
		return LoginResponse{}, errors.New(fmt.Sprintln("Error decoding JSON response: "+err.Error(), "body:", string(body)))
	}

	return loginResponse, err
}

func GenerateCouponPPG(couponData CouponInput) (string, error) {
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
		return "", err
	}

	req, err := http.NewRequest("POST", couponData.URL+"/api/master/v1/coupon/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+couponData.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintln("API request failed with status code:", resp.StatusCode, "body:", string(body)))
	}

	var coupon CouponResponse
	if err := json.Unmarshal(body, &coupon); err != nil {
		return "", err
	}

	if coupon.Description == "Could not find the offer in AI" {
		//lint:ignore ST1005 error strings should not be capitalized
		return "", errors.New("Product value is not found")
	}

	if coupon.Status != 1 {
		return "", errors.New(coupon.Description)
	}

	return strings.ReplaceAll(coupon.Result.Coupon, "@ppg", ""), nil
}

func GenerateCouponDF(couponData GenerateCouponDFRequest) (int, error) {
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
		return 0, err
	}

	req, err := http.NewRequest("POST", couponData.URL+"/api/get-voucher-outlet", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}

	req.Header.Set("key", couponData.Username)
	req.Header.Set("secret", couponData.Password)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, errors.New(fmt.Sprintln("API request failed with status code:", resp.StatusCode, "body:", string(body), " Request body: "+string(jsonData)))
	}

	var coupon DFGeneratecouponResponse
	if err := json.Unmarshal(body, &coupon); err != nil {
		return 0, errors.New(fmt.Sprintln("Error decoding JSON response: "+err.Error(), "body:", string(body), " Request body: "+string(jsonData)))
	}

	if !coupon.Status {
		return 0, errors.New("Failed to generate coupon body: " + string(body) + " Request body: " + string(jsonData))
	}

	return coupon.Data.VoucherCode, nil
}

func LoginEveryLounge(req LoginRequest) (LoginResponse, error) {

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
		return LoginResponse{}, err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(request)
	if err != nil {
		return LoginResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return LoginResponse{}, errors.New("API request failed with status code: " + res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return LoginResponse{}, err
	}

	var loginResponse AALoginResponse
	err = json.Unmarshal(body, &loginResponse)

	return LoginResponse{
		Token:   loginResponse.AccessToken,
		Expires: time.Now().Add(time.Duration(loginResponse.Expires) * time.Second).Format(time.RFC3339),
	}, err
}

func CreateOrderAllAirports(couponData AAGenerateCouponRequest) (map[string]interface{}, error) {

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
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, couponData.URL+"/api/v0/orders", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+couponData.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, errors.New(fmt.Sprintln("API request failed with status code:", resp.StatusCode, "body:", string(body)))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response, nil
}

func PayAllAirports(request AAGenerateCouponRequest) (map[string]interface{}, error) {

	if request.OrderID == 0 {
		return nil, errors.New("ResourceID and OrderID are required")
	}

	var payload = `{
		"mode": "Offline",
		"type": "Card"
	}`

	var url = fmt.Sprintf("%s/api/v0/orders/%d/pay", request.URL, request.OrderID)
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(payload))
	if err != nil {
		return nil, errors.New("Error creating request: " + err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+request.Token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("Error sending request: " + err.Error())
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, errors.New("API request failed with status code: " + string(body))
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, errors.New("Error decoding JSON response: " + err.Error())
	}

	return response, nil
}

func GetResourceID(request AAGenerateCouponRequest) (map[string]interface{}, error) {

	if request.AACode == "" {
		return nil, errors.New("ResourceID and AA Code are required")
	}

	var url = fmt.Sprintf("%s/api/v0/resources/%s", request.URL, request.AACode)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("Error creating request: " + err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+request.Token)

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("Error sending request: " + err.Error())
	}
	defer resp.Body.Close()

	var response map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, errors.New("API request failed with status code: " + string(body))
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, errors.New("Error decoding JSON response: " + err.Error())
	}

	return response, nil
}

func LoginHighPass(req LoginRequest) (LoginResponse, error) {

	url := req.URL + "/api/v1/token"

	payload := strings.NewReader(
		"grant_type=client_credentials" +
			"&client_type=ThirdParty" +
			"&api_key=" + req.AiShortCode)

	client := &http.Client{
		Timeout: 40 * time.Second,
	}
	request, err := http.NewRequest("POST", url, payload)

	if err != nil {
		return LoginResponse{}, err
	}

	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(request)
	if err != nil {
		return LoginResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return LoginResponse{}, errors.New("API request failed with status code: " + res.Status)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return LoginResponse{}, err
	}

	var loginResponse HighPassLoginResponse
	err = json.Unmarshal(body, &loginResponse)

	return LoginResponse{
		Token:   loginResponse.AccessToken,
		Expires: time.Now().Add(time.Duration(loginResponse.ExpiresIn) * time.Second).Format(time.RFC3339),
	}, err
}

func CreateISGService(reqData ISGServiceRequest) (ISGServiceResponse, error) {

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
		return ISGServiceResponse{}, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ISGServiceResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ISGServiceResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ISGServiceResponse{}, errors.New(fmt.Sprintln("API request failed with status code:", resp.StatusCode, "URL:", fullURL, "body:", string(body)))
	}

	var result ISGServiceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ISGServiceResponse{}, errors.New(fmt.Sprintln("Failed to unmarshal response error:", err, "URL:", fullURL, "body:", string(body)))
	}

	if result.Error {
		return result, errors.New("ISG Service Error: " + result.ErrorMessage + " URL: " + fullURL + " body: " + string(body))
	}

	return result, nil
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
