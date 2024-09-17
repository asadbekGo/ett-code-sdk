package ettcodesdk

import (
	"context"
	"net/http"
	"net/url"
)

type (
	NewRequestBody struct {
		Data        map[string]interface{} `json:"data"`
		RequestData struct {
			Method  string      `json:"method"`
			Path    string      `json:"path"`
			Headers http.Header `json:"headers"`
			Params  url.Values  `json:"params"`
			Body    []byte      `json:"body"`
		} `json:"request_data"`
		Auth struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		} `json:"auth"`
		Env struct {
			AdminBaseURL  string `json:"admin_base_url"`
			AgentBaseURL  string `json:"agent_base_url"`
			AuthBaseURL   string `json:"auth_base_url"`
			CDNBaseURL    string `json:"cdn_base_url"`
			BranchName    string `json:"branch_name"`
			EnvironmentID string `json:"environment_id"`
			ProjectID     string `json:"project_id"`
			ResourceEnvID string `json:"resource_env_id"`
			ResourceID    string `json:"resource_id"`
			SecretKey     string `json:"secret_key"`
		}
	}

	Request struct {
		Data map[string]interface{} `json:"data"`
	}

	Argument struct {
		Ctx               context.Context `json:"context"`
		AppId             string          `json:"app_id"`
		TableSlug         string          `json:"table_slug"`
		Request           Request         `json:"request"`
		DisableFaas       bool            `json:"disable_faas"`
		BlockCached       bool            `json:"block_cached"`
		BlockBuilder      bool            `json:"block_builder"`
		BlockedLoginTable bool            `json:"blocked_login_table"`
	}

	Data struct {
		AppId      string                 `json:"app_id"`
		Method     string                 `json:"method"`
		ObjectData map[string]interface{} `json:"object_data"`
		ObjectIds  []string               `json:"object_ids"`
		TableSlug  string                 `json:"table_slug"`
		UserId     string                 `json:"user_id"`
	}
)

// Response structures
type (
	// Create function response body >>>>> CREATE
	Datas struct {
		Data struct {
			Data struct {
				Data map[string]interface{} `json:"data"`
			} `json:"data"`
		} `json:"data"`
	}

	// ClientApiResponse This is get single api response >>>>> GET_SINGLE_BY_ID, GET_SLIM_BY_ID
	ClientApiResponse struct {
		Data ClientApiData `json:"data"`
	}

	ClientApiData struct {
		Data ClientApiResp `json:"data"`
	}

	ClientApiResp struct {
		Response map[string]interface{} `json:"response"`
	}

	Response struct {
		Status     string                 `json:"status"`
		Error      string                 `json:"error"`
		Data       map[string]interface{} `json:"data"`
		Attributes map[string]interface{} `json:"attributes"`
		Server     map[string]interface{} `json:"server"`
	}

	// GetListClientApiResponse This is get list api response >>>>> GET_LIST, GET_LIST_SLIM
	GetListClientApiResponse struct {
		Data GetListClientApiData `json:"data"`
	}

	GetListClientApiData struct {
		Data GetListClientApiResp `json:"data"`
	}

	GetListClientApiResp struct {
		Count    int                      `json:"count"`
		Response []map[string]interface{} `json:"response"`
	}

	// ClientApiUpdateResponse This is single update api response >>>>> UPDATE
	ClientApiUpdateResponse struct {
		Status      string `json:"status"`
		Description string `json:"description"`
		Data        struct {
			TableSlug string                 `json:"table_slug"`
			Data      map[string]interface{} `json:"data"`
		} `json:"data"`
	}

	// ClientApiMultipleUpdateResponse This is multiple update api response >>>>> MULTIPLE_UPDATE
	ClientApiMultipleUpdateResponse struct {
		Status      string `json:"status"`
		Description string `json:"description"`
		Data        struct {
			Data struct {
				Objects []map[string]interface{} `json:"objects"`
			} `json:"data"`
		} `json:"data"`
	}

	ResponseStatus struct {
		Status string `json:"status"`
	}

	WaitGroupError struct {
		Code         int
		Message      string
		ErrorMessage string
	}

	ResponseError struct {
		StatusCode         int
		Description        interface{}
		ErrorMessage       string
		ClientErrorMessage string
	}
)
