package datahash

type (
	DataHash struct {
		ProductVariantID   string              `json:"product_variant_id"`
		AdultPrice         Price               `json:"adultPrice"`
		ChildPrice         Price               `json:"childPrice"`
		Refund             Refund              `json:"refund"`
		UnavailablePeriods []UnavailablePeriod `json:"unavailablePeriods,omitempty"`
	}

	DataHashDelayCare struct {
		FlightNumber       string              `json:"flightNumber"`
		Date               string              `json:"date"`
		ProductVariantID   string              `json:"product_variant_id"`
		AdultPrice         Price               `json:"adultPrice"`
		ChildPrice         Price               `json:"childPrice"`
		Refund             Refund              `json:"refund"`
		UnavailablePeriods []UnavailablePeriod `json:"unavailablePeriods,omitempty"`
		Restrictions       []Restriction       `json:"restrictions,omitempty"`
	}

	Restriction struct {
		TypeId  string `json:"typeId"`
		ValueId string `json:"valueId"`
	}

	Price struct {
		PaxType    string  `json:"paxType"`
		GrossPrice float64 `json:"grossPrice"`
		NetPrice   float64 `json:"netPrice,omitempty"`
	}

	Refund struct {
		Type  string `json:"type"`
		Value int    `json:"value"`
	}

	UnavailablePeriod struct {
		Reason    string `json:"reason"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
)