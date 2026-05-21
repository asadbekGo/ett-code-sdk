package sabre

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

type UserResponse struct {
	PCC       string `json:"pcc"`
	PNR       string `json:"pnr"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func ParseToken(token string) (UserResponse, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(token)
		if err != nil {
			return UserResponse{}, errors.New("invalid base64 token: " + err.Error())
		}
	}

	var user UserResponse
	if err = json.Unmarshal(decoded, &user); err != nil {
		return UserResponse{}, errors.New("invalid token payload: " + err.Error())
	}

	if user.PCC == "" {
		return UserResponse{}, errors.New("missing required field: pcc")
	}
	if user.PNR == "" {
		return UserResponse{}, errors.New("missing required field: pnr")
	}
	if user.FirstName == "" {
		return UserResponse{}, errors.New("missing required field: firstName")
	}
	if user.LastName == "" {
		return UserResponse{}, errors.New("missing required field: lastName")
	}

	return user, nil
}
