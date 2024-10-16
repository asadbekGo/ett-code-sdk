package ettcodesdk

type Config struct {
	AppId         string
	BaseURL       string
	BotToken      string
	AccountIds    []string
	FunctionName  string
	ProjectId     string
	EnvironmentId string
	BranchName    string
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

func (cfg *Config) SetProjectId(projectId string) {
	cfg.ProjectId = projectId
}

func (cfg *Config) SetEnvorinmentId(environmentId string) {
	cfg.EnvironmentId = environmentId
}

func (cfg *Config) SetBranchName(branchName string) {
	cfg.BranchName = branchName
}
