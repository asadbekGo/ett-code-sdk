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
		url           = fmt.Sprintf("%s/v1/object/%s?from-ofs=%t&block_builder=%t&blocked_login_table=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder, arg.BlockedLoginTable)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	createObjectResponseInByte, err := DoRequest(url, "POST", arg.Request, appId)
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
		url          = fmt.Sprintf("%s/v1/object/%s?from-ofs=%t&block_builder=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	updateObjectResponseInByte, err := DoRequest(url, "PUT", arg.Request, appId)
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
		url                  = fmt.Sprintf("%s/v1/object/multiple-update/%s?from-ofs=%t&block_builder=%t", o.Cfg.BaseURL, arg.TableSlug, arg.DisableFaas, arg.BlockBuilder)
	)

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	multipleUpdateObjectsResponseInByte, err := DoRequest(url, "PUT", arg.Request, appId)
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

	getListResponseInByte, err := DoRequest(url, "POST", arg.Request, appId)
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
	}

	url = fmt.Sprintf("%s&data=%s", url, httpUrl.QueryEscape(string(reqObject)))
	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListResponseInByte, err := DoRequest(url, "GET", nil, appId)
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
	}

	var appId = o.Cfg.AppId
	if arg.AppId != "" {
		appId = arg.AppId
	}

	getListAggregateResponseInByte, err := DoRequest(url, "POST", arg.Request, appId)
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

	resByte, err := DoRequest(url, "GET", nil, appId)
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

	resByte, err := DoRequest(url, "GET", nil, appId)
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

	_, err := DoRequest(url, "DELETE", nil, appId)
	if err != nil {
		response.Data = map[string]interface{}{"message": "Error while deleting object", "error": err.Error()}
		response.Status = "error"
		return response, err
	}

	return response, nil
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

	_, err := DoRequest(url, "DELETE", arg.Request.Data, appId)
	if err != nil {
		response.Data = map[string]interface{}{"message": "Error while deleting objects", "error": err.Error()}
		response.Status = "error"
		return response, err
	}

	return response, nil
}

func (o *ObjectFunction) SendTelergram(text string) error {
	client := &http.Client{}

	if ContainsLike(Mode, text) {
		text = strings.Replace(text, "\n", "", -1)
	} else {
		text = o.Cfg.FunctionName + " >>> " + time.Now().Format(time.RFC3339) + " >>>>> " + text
	}

	for _, e := range o.Cfg.AccountIds {
		botUrl := fmt.Sprintf("https://api.telegram.org/bot"+o.Cfg.BotToken+"/sendMessage?chat_id="+e+"&text=%s", text)
		request, err := http.NewRequest("GET", botUrl, nil)
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
