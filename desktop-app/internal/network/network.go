package network

type Mode string

const (
	ModeUnconfigured Mode = "unconfigured"
	ModeLAN          Mode = "lan"
	ModePublicURL    Mode = "public-url"
	ModeManagedBeta  Mode = "managed-beta"
)
