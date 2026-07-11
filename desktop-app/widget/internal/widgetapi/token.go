package widgetapi

type TokenSource interface{ Token() (string, error) }

type NoTokenSource struct{}

func (NoTokenSource) Token() (string, error) { return "", ErrCredentialMissing }
