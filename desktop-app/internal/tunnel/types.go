package tunnel

type Identity struct {
	InstallID              string `json:"installId"`
	MachineFingerprintHash string `json:"machineFingerprintHash"`
}

type Binding struct {
	PublicBaseURL string `json:"publicBaseUrl"`
	TunnelID      string `json:"tunnelId"`
	TokenVersion  int    `json:"tokenVersion"`
	RedeemedAt    string `json:"redeemedAt"`
}

type HealthCheck struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	Endpoint  string `json:"endpoint,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

type Status struct {
	State               string       `json:"state"`
	Message             string       `json:"message"`
	Configured          bool         `json:"configured"`
	Running             bool         `json:"running"`
	TokenExpired        bool         `json:"tokenExpired"`
	PublicBaseURL       string       `json:"publicBaseUrl,omitempty"`
	TunnelID            string       `json:"tunnelId,omitempty"`
	TokenVersion        int          `json:"tokenVersion,omitempty"`
	RedeemedAt          string       `json:"redeemedAt,omitempty"`
	TokenFingerprint    string       `json:"tokenFingerprint,omitempty"`
	ResolvedBinary      string       `json:"resolvedBinary,omitempty"`
	BinarySource        string       `json:"binarySource,omitempty"`
	StartedAt           string       `json:"startedAt,omitempty"`
	LastRedeemErrorCode string       `json:"lastRedeemErrorCode,omitempty"`
	SharedProcessNotice string       `json:"sharedProcessNotice,omitempty"`
	HealthProbePath     string       `json:"healthProbePath,omitempty"`
	RecentLogCount      int          `json:"recentLogCount"`
	LastHealth          *HealthCheck `json:"lastHealth,omitempty"`
}

type LogLine struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type TunnelCredentials struct {
	AccountTag   string `json:"AccountTag"`
	TunnelSecret string `json:"TunnelSecret"`
	TunnelID     string `json:"TunnelID"`
	Endpoint     string `json:"Endpoint"`
}

type RedeemResponse struct {
	PublicBaseURL     string             `json:"publicBaseUrl"`
	TunnelToken       string             `json:"tunnelToken"`
	TunnelCredentials *TunnelCredentials `json:"tunnelCredentials,omitempty"`
	TunnelID          string             `json:"tunnelId"`
	TokenVersion      int                `json:"tokenVersion"`
	IssuedAt          string             `json:"issuedAt"`
}

type WatchBootstrapConfigRequest struct {
	BootstrapCode string `json:"bootstrapCode"`
	Environment   string `json:"environment"`
	APIBase       string `json:"apiBase"`
	Source        string `json:"source,omitempty"`
}

type WatchBootstrapConfig struct {
	Environment  string `json:"environment"`
	APIBase      string `json:"apiBase"`
	Source       string `json:"source"`
	ConfiguredAt string `json:"configuredAt"`
}

type WatchBootstrapConfigResponse struct {
	BootstrapCode string               `json:"bootstrapCode"`
	Status        string               `json:"status"`
	Config        WatchBootstrapConfig `json:"config"`
}
