package port

type Config struct {
	Server     ServerConfig      `json:"server"`
	Store      StoreConfig       `json:"store"`
	Retrievers []RetrieverConfig `json:"retrievers"`
	LLM        LLMConfig         `json:"llm"`
	Dedup      DedupConfig       `json:"dedup,omitempty"`
	Maintain   MaintainConfig    `json:"maintain,omitempty"`
}

type ServerConfig struct {
	Addr        string   `json:"addr"`
	CORSOrigins []string `json:"cors_origins,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
}

type StoreConfig struct {
	Type                   string `json:"type"` // postgres
	DSN                    string `json:"dsn"`
	MaxOpenConns           int    `json:"max_open_conns,omitempty"`
	MaxIdleConns           int    `json:"max_idle_conns,omitempty"`
	ConnMaxLifetimeSeconds int    `json:"conn_max_lifetime_seconds,omitempty"`
}

type RetrieverConfig struct {
	Type    string                 `json:"type"` // keyword / fts / vector
	Enabled bool                   `json:"enabled"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type LLMConfig struct {
	BaseURL             string `json:"base_url"`
	APIKey              string `json:"api_key"`
	ChatModel           string `json:"chat_model"`
	EmbeddingModel      string `json:"embedding_model"`
	EmbeddingDimension  int    `json:"embedding_dimension,omitempty"`
	ChatTimeoutSeconds  int    `json:"chat_timeout_seconds,omitempty"`
	EmbedTimeoutSeconds int    `json:"embed_timeout_seconds,omitempty"`
}

type DedupConfig struct {
	ReinforceThreshold float64 `json:"reinforce_threshold,omitempty"`
	RelateThreshold    float64 `json:"relate_threshold,omitempty"`
}

type MaintainConfig struct {
	LinkThreshold           float64 `json:"link_threshold,omitempty"`
	LinkScanLimit           int     `json:"link_scan_limit,omitempty"`
	ConsolidationMinCluster int     `json:"consolidation_min_cluster,omitempty"`
	ConsolidationScanLimit  int     `json:"consolidation_scan_limit,omitempty"`
	DecayDays               int     `json:"decay_days,omitempty"`
	DecayScanLimit          int     `json:"decay_scan_limit,omitempty"`
	TagClusterScanLimit     int     `json:"tag_cluster_scan_limit,omitempty"`
}

type ConfigLoader interface {
	Load(path string) (*Config, error)
}
