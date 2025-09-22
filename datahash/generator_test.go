package datahash

import "testing"

type (
	ValidateDataHashTestCase struct {
		name           string
		dataHashBody   interface{}
		clientDatahash string
		expectError    bool
	}
)

var ValidateDataHashTestCases = []ValidateDataHashTestCase{
	// test cases for lounge Datahash
	{
		name: "Valid case",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000593",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 30,
				NetPrice:   27,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 30,
				NetPrice:   27,
			},
			Refund: Refund{
				Type:  "day",
				Value: 0,
			},
			UnavailablePeriods: []UnavailablePeriod{
				{
					Reason:    "test",
					StartDate: "18.09.2025 20:53",
					EndDate:   "09.10.2025 20:53",
				},
			},
		},
		clientDatahash: "25e7ed60dad79cfc69e3754dd362846d8eb75d3b822ac7adb149bcee8ed7e998",
		expectError:    false,
	},
	{
		name: "Client sending invalid datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000593",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 30,
				NetPrice:   27,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 30,
				NetPrice:   27,
			},
			Refund: Refund{
				Type:  "day",
				Value: 0,
			},
			UnavailablePeriods: []UnavailablePeriod{
				{
					Reason:    "test",
					StartDate: "18.09.2025 20:53",
					EndDate:   "09.10.2025 20:53",
				},
			},
		},
		clientDatahash: "25e7ed60dad79cfc69e3754dd362846d8eb75d3b822ac7adb149bcee8ed7e999",
		expectError:    true,
	},
	{
		name: "Server generating invalid datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-0000005931",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 30,
				NetPrice:   27,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 30,
				NetPrice:   27,
			},
			Refund: Refund{
				Type:  "day",
				Value: 0,
			},
			UnavailablePeriods: []UnavailablePeriod{
				{
					Reason:    "test",
					StartDate: "18.09.2025 20:53",
					EndDate:   "09.10.2025 20:53",
				},
			},
		},
		clientDatahash: "25e7ed60dad79cfc69e3754dd362846d8eb75d3b822ac7adb149bcee8ed7e998",
		expectError:    true,
	},
	// test cases for delayCare Datahash
	{
		name: "Valid case for delayCare Datahash",
		dataHashBody: DataHashDelayCare{
			FlightNumber:     "EK8",
			Date:             "2025-08-30",
			ProductVariantID: "PV-000000621",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 5,
				NetPrice:   3,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 5,
				NetPrice:   3,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
			UnavailablePeriods: []UnavailablePeriod{},
			Restrictions:       []Restriction{},
		},
		clientDatahash: "ef268ec8df1f9fa974a9e33e8ef3db53409812dbd1dede49d9354cb03c5f2d46",
		expectError:    false,
	},
	{
		name: "Client sending invalid datahash for delayCare Datahash",
		dataHashBody: DataHashDelayCare{
			FlightNumber:     "EK8",
			Date:             "2025-08-30",
			ProductVariantID: "PV-000000621",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 5,
				NetPrice:   3,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 5,
				NetPrice:   3,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
			UnavailablePeriods: []UnavailablePeriod{},
			Restrictions:       []Restriction{},
		},
		clientDatahash: "ef268ec8df1f9fa974a9e33e8ef3db53409812dbd1dede49d9354cb03c5f2d47",
		expectError:    true,
	},
	{
		name: "server generating invalid Datahash",
		dataHashBody: DataHashDelayCare{
			FlightNumber:     "EK8",
			Date:             "2025-08-30",
			ProductVariantID: "PV-0000006211",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 5,
				NetPrice:   3,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 5,
				NetPrice:   3,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
			UnavailablePeriods: []UnavailablePeriod{},
			Restrictions:       []Restriction{},
		},
		clientDatahash: "ef268ec8df1f9fa974a9e33e8ef3db53409812dbd1dede49d9354cb03c5f2d46",
		expectError:    true,
	},
	// test cases for fastTrack Datahash
	{
		name: "Valid case for fastTrack Datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000623",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 75,
				NetPrice:   75,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 75,
				NetPrice:   75,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
		},
		clientDatahash: "ac3b70fb7d9d174f25914449408a29a813d51c7e2070443513c88912c282f1d5",
		expectError:    false,
	},
	{
		name: "Client sending invalid datahash for fastTrack Datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000623",
			AdultPrice: Price{
				PaxType:    "adult",
				NetPrice:   75,
				GrossPrice: 75,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 75,
				NetPrice:   75,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
		},
		clientDatahash: "ac3b70fb7d9d174f25914449408a29a813d51c7e2070443513c88912c282f1d56",
		expectError:    true,
	},
	{
		name: "server generating invalid Datahash for fastTrack Datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-0000006231",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 75,
				NetPrice:   75,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 75,
				NetPrice:   75,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
		},
		clientDatahash: "ac3b70fb7d9d174f25914449408a29a813d51c7e2070443513c88912c282f1d5",
		expectError:    true,
	},
}

func TestValidateDataHash(t *testing.T) {
	for _, tc := range ValidateDataHashTestCases {
		t.Run(tc.name, func(t *testing.T) {
			isDatahashValid, _ := ValidateDataHash(tc.dataHashBody, tc.clientDatahash)
			if !isDatahashValid && !tc.expectError {
				t.Errorf("Expected data hash to be valid, but got invalid")
			}
			if isDatahashValid && tc.expectError {
				t.Errorf("Expected data hash to be invalid, but got valid")
			}
		})
	}
}

// Test cases for GenerateDataHash

type (
	GenerateDataHashTestCase struct {
		name             string
		dataHashBody     interface{}
		expectedDatahash string
		expectError      bool
	}
)

var GenerateDataHashTestCases = []GenerateDataHashTestCase{
	// test cases for lounge Datahash
	{
		name: "Valid case",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000593",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 30,
				NetPrice:   27,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 30,
				NetPrice:   27,
			},
			Refund: Refund{
				Type:  "day",
				Value: 0,
			},
			UnavailablePeriods: []UnavailablePeriod{
				{
					Reason:    "test",
					StartDate: "18.09.2025 20:53",
					EndDate:   "09.10.2025 20:53",
				},
			},
		},
		expectedDatahash: "25e7ed60dad79cfc69e3754dd362846d8eb75d3b822ac7adb149bcee8ed7e998",
		expectError:      false,
	},
	// test cases for delayCare Datahash
	{
		name: "Valid case for delayCare Datahash",
		dataHashBody: DataHashDelayCare{
			FlightNumber:     "EK8",
			Date:             "2025-08-30",
			ProductVariantID: "PV-000000621",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 5,
				NetPrice:   3,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 5,
				NetPrice:   3,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
			UnavailablePeriods: []UnavailablePeriod{},
			Restrictions:       []Restriction{},
		},
		expectedDatahash: "ef268ec8df1f9fa974a9e33e8ef3db53409812dbd1dede49d9354cb03c5f2d46",
		expectError:      false,
	},
	// test cases for fastTrack Datahash
	{
		name: "Valid case for fastTrack Datahash",
		dataHashBody: DataHash{
			ProductVariantID: "PV-000000623",
			AdultPrice: Price{
				PaxType:    "adult",
				GrossPrice: 75,
				NetPrice:   75,
			},
			ChildPrice: Price{
				PaxType:    "child",
				GrossPrice: 75,
				NetPrice:   75,
			},
			Refund: Refund{
				Type:  "day",
				Value: 1,
			},
		},
		expectedDatahash: "ac3b70fb7d9d174f25914449408a29a813d51c7e2070443513c88912c282f1d5",
		expectError:      false,
	},
}

func TestGenerateDataHash(t *testing.T) {
	for _, tc := range GenerateDataHashTestCases {
		t.Run(tc.name, func(t *testing.T) {
			generatedDatahash, err := GenerateDataHash(tc.dataHashBody)
			if err != nil && !tc.expectError {
				t.Errorf("Expected no error, but got: %v", err)
			}
			if err == nil && tc.expectError {
				t.Errorf("Expected error, but got none")
			}
			if generatedDatahash != tc.expectedDatahash && !tc.expectError {
				t.Errorf("Expected data hash: %s, but got: %s", tc.expectedDatahash, generatedDatahash)
			}
		})
	}
}
