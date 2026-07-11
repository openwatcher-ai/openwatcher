package widgetprefs

type Position struct {
	MonitorID  string  `json:"monitorId"`
	Edge       string  `json:"edge"`
	Normalized float64 `json:"normalized"`
}
type Store interface {
	Load() Position
	Save(Position) error
}
type memoryStore struct{ p Position }

func NewMemoryStore() Store                  { return &memoryStore{} }
func (m *memoryStore) Load() Position        { return m.p }
func (m *memoryStore) Save(p Position) error { m.p = p; return nil }
