package port

type Config struct {
	Server     ServerConfig      `json:"server"`
	Store      StoreConfig       `json:"store"`
	Retrievers []RetrieverConfig `json:"retrievers"`
	LLM        LLMConfig         `json:"llm"`
}

type ServerConfig struct {
	Addr        string   `json:"addr"`
	CORSOrigins []string `json:"cors_origins,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
}

type StoreConfig struct {
	Type string `json:"type"` // postgres
	DSN  string `json:"dsn"`
}

type RetrieverConfig struct {
	Type    string                 `json:"type"` // keyword / fts / vector
	Enabled bool                   `json:"enabled"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type LLMConfig struct {
	BaseURL            string `json:"base_url"`
	APIKey             string `json:"api_key"`
	ChatModel          string `json:"chat_model"`
	EmbeddingModel     string `json:"embedding_model"`
	EmbeddingDimension int    `json:"embedding_dimension,omitempty"`
}

type ConfigLoader interface {
	Load(path string) (*Config, error)
}
