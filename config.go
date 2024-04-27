package ettcodesdk

type Config struct {
	AppId        string
	BaseURL      string
	BotToken     string
	AccountIds   []string
	FunctionName string
}

func (cfg *Config) SetAppId(appId string) {
	cfg.AppId = appId
}

func (cfg *Config) SetBaseUrl(url string) {
	cfg.BaseURL = url
}

func (cfg *Config) SetBotToken(token string) {
	cfg.BotToken = token
}

func (cfg *Config) SetAccountIds(accountIds []string) {
	cfg.AccountIds = accountIds
}

func (cfg *Config) SetFunctionName(functionName string) {
	cfg.FunctionName = functionName
}
