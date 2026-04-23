package jsonfile

import (
	"encoding/json"
	"os"

	"github.com/personal-know/internal/port"
)

type Loader struct{}

func New() *Loader { return &Loader{} }

func (l *Loader) Load(path string) (*port.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg port.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultConfig() *port.Config {
	return &port.Config{
		Server: port.ServerConfig{Addr: ":8081"},
		Store: port.StoreConfig{
			Type: "postgres",
		},
		Retrievers: []port.RetrieverConfig{
			{Type: "keyword", Enabled: true},
			{Type: "fts", Enabled: true},
			{Type: "vector", Enabled: true},
		},
		LLM: port.LLMConfig{
			ChatModel:      "gpt-4o-mini",
			EmbeddingModel: "text-embedding-3-small",
		},
	}
}
