package ettcodesdk

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpUrl "net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cast"
	tgbotapiK "gopkg.in/telegram-bot-api.v4"
)

type ObjectFunction struct {
	Cfg    *Config
	Logger *FaasLogger
}

func New(cfg *Config) *ObjectFunction {
	return &ObjectFunction{
		Cfg:    cfg,
		Logger: NewLoggerFunction(cfg.FunctionName),
	}
}

func (o *ObjectFunction) CreateObject(arg *Argument) (Datas, Response, error) {
	var (
		response      = Response{Status: "done"}
		createdObject = Datas{}
		url           = fmt.Sprintf("%s/v1/object/%s?from-ofs=%t&block_builder=%t&blocked_login_table=%t&block_cache_clear=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder, arg.BlockedLoginTable, arg.BlockCacheClear)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	createObjectResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPost, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(createObjectResponseInByte), "message": "Can't send request", "error": err.Error()}
		response.Status = "error"
		return Datas{}, response, err
	}

	err = json.Unmarshal(createObjectResponseInByte, &createdObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(createObjectResponseInByte), "message": "Error while unmarshalling create object", "error": err.Error()}
		response.Status = "error"
		return Datas{}, response, err
	}

	return createdObject, response, nil
}

func (o *ObjectFunction) UpdateObject(arg *Argument) (ClientApiUpdateResponse, Response, error) {
	var (
		response     = Response{Status: "done"}
		updateObject = ClientApiUpdateResponse{}
		url          = fmt.Sprintf("%s/v1/object/%s?from-ofs=%t&block_builder=%t&block_cache_clear=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder, arg.BlockCacheClear)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	updateObjectResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPut, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(updateObjectResponseInByte), "message": "Error while updating object", "error": err.Error()}
		response.Status = "error"
		return ClientApiUpdateResponse{}, response, err
	}

	err = json.Unmarshal(updateObjectResponseInByte, &updateObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(updateObjectResponseInByte), "message": "Error while unmarshalling update object", "error": err.Error()}
		response.Status = "error"
		return ClientApiUpdateResponse{}, response, err
	}

	return updateObject, response, nil
}

func (o *ObjectFunction) MultipleUpdate(arg *Argument) (ClientApiMultipleUpdateResponse, Response, error) {
	var (
		response             = Response{Status: "done"}
		multipleUpdateObject = ClientApiMultipleUpdateResponse{}
		url                  = fmt.Sprintf("%s/v1/object/multiple-update/%s?from-ofs=%t&block_builder=%t&block_cache_clear=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder, arg.BlockCacheClear)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	if arg.IgnoreDuplicateGuid {
		url += "&ignore_duplicate_guid=true"
	}

	multipleUpdateObjectsResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPut, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(multipleUpdateObjectsResponseInByte), "message": "Error while multiple updating objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	err = json.Unmarshal(multipleUpdateObjectsResponseInByte, &multipleUpdateObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(multipleUpdateObjectsResponseInByte), "message": "Error while unmarshalling multiple update objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	return multipleUpdateObject, response, nil
}

func (o *ObjectFunction) UpdateManyQuery(arg *Argument) (ClientApiMultipleUpdateResponse, Response, error) {
	var (
		response         = Response{Status: "done"}
		updateManyObject = ClientApiMultipleUpdateResponse{}
		url              = fmt.Sprintf("%s/v1/object/update-many-query/%s", o.Cfg.BaseURL, arg.TableSlug)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	updateManyObjectsResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPut, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(updateManyObjectsResponseInByte), "message": "Error while update many query objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	err = json.Unmarshal(updateManyObjectsResponseInByte, &updateManyObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(updateManyObjectsResponseInByte), "message": "Error while unmarshalling update many query objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	return updateManyObject, response, nil
}

func (o *ObjectFunction) GetList(arg *Argument) (GetListClientApiResponse, Response, error) {
	var (
		response      Response
		getListObject GetListClientApiResponse
		url           = fmt.Sprintf("%s/v1/object/get-list/%s?from-ofs=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas)
		page, limit   int
	)

	if _, ok := arg.Request.Data["page"].(int); ok {
		page = arg.Request.Data["page"].(int)
	}

	if _, ok := arg.Request.Data["limit"]; ok {
		limit = arg.Request.Data["limit"].(int)
	}

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		limit = 10
	}

	arg.Request.Data["offset"] = (page - 1) * limit
	arg.Request.Data["limit"] = limit

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPost, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListResponseInByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	err = json.Unmarshal(getListResponseInByte, &getListObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListResponseInByte), "message": "Error while unmarshalling get list object", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	return getListObject, response, nil
}

func (o *ObjectFunction) GetListSlim(arg *Argument) (GetListClientApiResponse, Response, error) {
	var (
		response    Response
		listSlim    GetListClientApiResponse
		url         = fmt.Sprintf("%s/v2/object-slim/get-list/%s?from-ofs=%t&block_cached=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockCached)
		page, limit int
	)

	reqObject, err := json.Marshal(arg.Request.Data)
	if err != nil {
		response.Data = map[string]interface{}{"message": "Error while marshalling request getting list slim object", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	if _, ok := arg.Request.Data["limit"]; ok {
		limit = arg.Request.Data["limit"].(int)
		url = fmt.Sprintf("%s&limit=%d", url, limit)
	}

	if _, ok := arg.Request.Data["page"].(int); ok {
		page = arg.Request.Data["page"].(int)
		url = fmt.Sprintf("%s&offset=%d", url, (page-1)*limit)
	} else if _, ok := arg.Request.Data["offset"].(int); ok {
		url = fmt.Sprintf("%s&offset=%d", url, arg.Request.Data["offset"].(int))
	}

	url = fmt.Sprintf("%s&data=%s", url, httpUrl.QueryEscape(string(reqObject)))
	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodGet, nil, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListResponseInByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	err = json.Unmarshal(getListResponseInByte, &listSlim)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListResponseInByte), "message": "Error while unmarshalling get list object", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	return listSlim, response, nil
}

func (o *ObjectFunction) GetListAggregate(arg *Argument) (GetListClientApiResponse, Response, error) {
	var (
		response         Response
		getListAggregate GetListClientApiResponse
		url              = fmt.Sprintf("%s/v1/object/get-list-aggregate/%s?from-ofs=%t&block_cached=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockCached)
		page, limit      int
	)

	if _, ok := arg.Request.Data["limit"]; ok {
		limit = arg.Request.Data["limit"].(int)
		url = fmt.Sprintf("%s&limit=%d", url, limit)
	}

	if _, ok := arg.Request.Data["page"].(int); ok {
		page = arg.Request.Data["page"].(int)
		url = fmt.Sprintf("%s&offset=%d", url, (page-1)*limit)
	} else if _, ok := arg.Request.Data["offset"].(int); ok {
		url = fmt.Sprintf("%s&offset=%d", url, arg.Request.Data["offset"].(int))
	}

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListAggregateResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPost, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListAggregateResponseInByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	err = json.Unmarshal(getListAggregateResponseInByte, &getListAggregate)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListAggregateResponseInByte), "message": "Error while unmarshalling get list object", "error": err.Error()}
		response.Status = "error"
		return GetListClientApiResponse{}, response, err
	}

	return getListAggregate, response, nil
}

func (o *ObjectFunction) GetQueryAggregation(arg *Argument) (ClientApiAggregationResponse, Response, error) {
	var (
		response         Response
		getListAggregate ClientApiAggregationResponse
		url              = fmt.Sprintf("%s/v2/items/%s/aggregation", o.Cfg.BaseURL, arg.TableSlug)
	)

	if arg.Request.Data == nil {
		response.Data = map[string]interface{}{"message": "Request data is nil"}
		response.Status = "error"
		return ClientApiAggregationResponse{}, response, fmt.Errorf("request data is nil")
	}

	if _, ok := arg.Request.Data["pipelines"]; !ok {
		response.Data = map[string]interface{}{"message": "Pipelines key not found in request data"}
		response.Status = "error"
		return ClientApiAggregationResponse{}, response, fmt.Errorf("pipelines key not found in request data")
	}

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListAggregateResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodPost, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListAggregateResponseInByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return ClientApiAggregationResponse{}, response, err
	}

	err = json.Unmarshal(getListAggregateResponseInByte, &getListAggregate)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(getListAggregateResponseInByte), "message": "Error while unmarshalling get query aggregation object", "error": err.Error()}
		response.Status = "error"
		return ClientApiAggregationResponse{}, response, err
	}

	return getListAggregate, response, nil
}

func (o *ObjectFunction) GetSingle(arg *Argument) (ClientApiResponse, Response, error) {
	var (
		response  Response
		getObject ClientApiResponse
		url       = fmt.Sprintf("%s/v1/object/%s/%v?from-ofs=%t", o.Cfg.BaseURL, arg.TableSlug, arg.Request.Data["guid"], arg.DisableFaas)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	resByte, err := DoRequest(arg.Ctx, url, http.MethodGet, nil, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(resByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return ClientApiResponse{}, response, err
	}

	err = json.Unmarshal(resByte, &getObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(resByte), "message": "Error while unmarshalling get list object", "error": err.Error()}
		response.Status = "error"
		return ClientApiResponse{}, response, err
	}

	return getObject, response, nil
}

func (o *ObjectFunction) GetSingleSlim(arg *Argument) (ClientApiResponse, Response, error) {
	var (
		response  Response
		getObject ClientApiResponse
		url       = fmt.Sprintf("%s/v1/object-slim/%s/%v?from-ofs=%t", o.Cfg.BaseURL, arg.TableSlug, arg.Request.Data["guid"], arg.DisableFaas)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	resByte, err := DoRequest(arg.Ctx, url, http.MethodGet, nil, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(resByte), "message": "Can't sent request", "error": err.Error()}
		response.Status = "error"
		return ClientApiResponse{}, response, err
	}

	err = json.Unmarshal(resByte, &getObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(resByte), "message": "Error while unmarshalling to object", "error": err.Error()}
		response.Status = "error"
		return ClientApiResponse{}, response, err
	}

	return getObject, response, nil
}

func (o *ObjectFunction) Delete(arg *Argument) (Response, error) {
	var (
		response = Response{
			Status: "done",
		}
		url = fmt.Sprintf("%s/v1/object/%s/%v?from-ofs=%t", o.Cfg.BaseURL, arg.TableSlug, arg.Request.Data["guid"], arg.DisableFaas)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	_, err := DoRequest(arg.Ctx, url, http.MethodDelete, nil, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"message": "Error while deleting object", "error": err.Error()}
		response.Status = "error"
		return response, err
	}

	return response, nil
}

func (o *ObjectFunction) DeleteManyQuery(arg *Argument) (ClientApiMultipleUpdateResponse, Response, error) {
	var (
		response         = Response{Status: "done"}
		deleteManyObject = ClientApiMultipleUpdateResponse{}
		url              = fmt.Sprintf("%s/v1/object/delete-many-query/%s", o.Cfg.BaseURL, arg.TableSlug)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	deleteManyObjectsResponseInByte, err := DoRequest(arg.Ctx, url, http.MethodDelete, arg.Request, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(deleteManyObjectsResponseInByte), "message": "Error while deleting many query objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	err = json.Unmarshal(deleteManyObjectsResponseInByte, &deleteManyObject)
	if err != nil {
		response.Data = map[string]interface{}{"description": string(deleteManyObjectsResponseInByte), "message": "Error while unmarshalling delete many query objects", "error": err.Error()}
		response.Status = "error"
		return ClientApiMultipleUpdateResponse{}, response, err
	}

	return deleteManyObject, response, nil
}

func (o *ObjectFunction) MultipleDelete(arg *Argument) (Response, error) {
	var (
		response = Response{Status: "done"}
		url      = fmt.Sprintf("%s/v1/object/%s/?from-ofs=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	_, err := DoRequest(arg.Ctx, url, http.MethodDelete, arg.Request.Data, appId, nil)
	if err != nil {
		response.Data = map[string]interface{}{"message": "Error while deleting objects", "error": err.Error()}
		response.Status = "error"
		return response, err
	}

	return response, nil
}

func (o *ObjectFunction) SendTelegram(text string) error {
	client := &http.Client{}

	if ContainsLike(Mode, text) {
		text = strings.Replace(text, "\n", "", -1)
	} else {
		text = o.Cfg.FunctionName + " >>> " + time.Now().Format(time.RFC3339) + " >>>>> " + text
	}

	if o.Cfg.BranchName != "" {
		text = strings.ToUpper(o.Cfg.BranchName) + " >>> " + text
	}

	for _, e := range o.Cfg.AccountIds {
		botUrl := fmt.Sprintf("https://api.telegram.org/bot"+o.Cfg.BotToken+"/sendMessage?chat_id="+e+"&text=%s", text)
		request, err := http.NewRequest(http.MethodGet, botUrl, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(request)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}

	return nil
}

func (o *ObjectFunction) SendTelegramFile(req []byte, filename string) error {
	err := os.WriteFile(filename, req, 0644)
	if err != nil {
		return err
	}
	defer os.Remove(filename)

	for _, e := range o.Cfg.AccountIds {
		bot, err := tgbotapiK.NewBotAPI(o.Cfg.BotToken)
		if err != nil {
			return err
		}

		message := tgbotapiK.NewDocumentUpload(cast.ToInt64(e), filename)
		_, err = bot.Send(message)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *ObjectFunction) Config() *Config {
	return o.Cfg
}
