package ets_hub

import (
	"fmt"
	"testing"
)

func TestSimpleEncoding(t *testing.T) {
	type args struct {
		secretKey string
		jsonData  string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Payze UZS",
			args: args{
				secretKey: "WUWj8Fie9CRAMgZDx5UEuqywXyRq4YlzeOz0FKhQn08bcCLlUG21tluBMBAdw1oe",
				jsonData:  `{"psp_id":3623,"amount":811778,"lifetime":5,"currency":"UZS","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Payze USD",
			args: args{
				secretKey: "WUWj8Fie9CRAMgZDx5UEuqywXyRq4YlzeOz0FKhQn08bcCLlUG21tluBMBAdw1oe",
				jsonData:  `{"psp_id":3623,"amount":811778,"lifetime":5,"currency":"USD","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Payze KZT",
			args: args{
				secretKey: "WUWj8Fie9CRAMgZDx5UEuqywXyRq4YlzeOz0FKhQn08bcCLlUG21tluBMBAdw1oe",
				jsonData:  `{"psp_id":3623,"amount":811778,"lifetime":5,"currency":"KZT","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Юкасса RUB",
			args: args{
				secretKey: "rjH3hJUqNdCiIRzbjP6vU9SUJZGT44p2nRMZyvtzzbt5HuwOCABUU23ZClPUmr6F",
				jsonData:  `{"psp_id":3625,"amount":811778,"lifetime":5,"currency":"RUB","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Юкасса USD",
			args: args{
				secretKey: "rjH3hJUqNdCiIRzbjP6vU9SUJZGT44p2nRMZyvtzzbt5HuwOCABUU23ZClPUmr6F",
				jsonData:  `{"psp_id":3625,"amount":811778,"lifetime":5,"currency":"KZT","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Uniteller RUB",
			args: args{
				secretKey: "tsOzri5fUK0dJ1oMtHJmMgjiVDua5v058xR6VKFC1WOVICCX0CdneB93JexoAYtp",
				jsonData:  `{"psp_id":3624,"amount":811778,"lifetime":5,"currency":"RUB","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Uniteller GBP",
			args: args{
				secretKey: "tsOzri5fUK0dJ1oMtHJmMgjiVDua5v058xR6VKFC1WOVICCX0CdneB93JexoAYtp",
				jsonData:  `{"psp_id":3624,"amount":811778,"lifetime":5,"currency":"GBP","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Uniteller USD",
			args: args{
				secretKey: "tsOzri5fUK0dJ1oMtHJmMgjiVDua5v058xR6VKFC1WOVICCX0CdneB93JexoAYtp",
				jsonData:  `{"psp_id":3624,"amount":811778,"lifetime":5,"currency":"USD","success_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers\/5wLhqiz7","fail_url":"https:\/\/test-widget.easyto.travel\/W-7vd094z2yugj\/vouchers","callback_url":"https:\/\/dev-test-api.easyto.travel\/ets_hub_webhooks\/test","details":{"order_id":"5wLhqiz7","email":"asadbek.ergashev@easyto.travel"}}`,
			},
		},
		{
			name: "Payze UZS Status",
			args: args{
				secretKey: "WUWj8Fie9CRAMgZDx5UEuqywXyRq4YlzeOz0FKhQn08bcCLlUG21tluBMBAdw1oe",
				jsonData:  `{"order_id":"15cace23-497f-4fa6-9770-5f78ed614b03","amount":42832400,"currency":"UZS"}`,
			},
		},
		{
			name: "Beepul UZS Refund",
			args: args{
				secretKey: "KyaGIiuWLfGjob3vroWaCz4qYiAQenDNUUvPCgDnTq3Z9Xe2M84JVrNAluBUTDOv",
				jsonData:  `{"order_id":"2109777f-54b2-4b2d-a528-5748747a606f","amount":50000,"currency":"UZS"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := SimpleEncoding(tt.args.secretKey, tt.args.jsonData)
			fmt.Println("Signature: ", signature)
		})
	}
}
