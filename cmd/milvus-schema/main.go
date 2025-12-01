package main

import (
	"bytes"
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/service/knowledge-base/etl/processor"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	int64Type       = "Int64"
	floatVectorType = "FloatVector"
	varcharType     = "VarChar"
)

type CreateCollectionRequest struct {
	CollectionName string         `json:"collectionName"`
	Schema         *Schema        `json:"schema"`
	IndexParams    []*IndexParams `json:"indexParams"`
}

type Schema struct {
	AutoID             bool     `json:"autoId"`
	EnableDynamicField bool     `json:"enableDynamicField"`
	Fields             []*Field `json:"fields"`
}

type Field struct {
	FieldName         string            `json:"fieldName"`
	DataType          string            `json:"dataType"`
	ElementTypeParams map[string]string `json:"elementTypeParams"`
	IsPrimary         bool              `json:"isPrimary,omitempty"`
}

type IndexParams struct {
	MetricType string            `json:"metricType"`
	FieldName  string            `json:"fieldName"`
	IndexName  string            `json:"indexName"`
	Params     map[string]string `json:"params"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := config.Cfg.Milvus.Endpoint + "/v2/vectordb/collections/create"

	fields := []*Field{
		{
			FieldName: "id",
			DataType:  int64Type,
			IsPrimary: true,
		},
		{
			FieldName: "vector",
			DataType:  floatVectorType,
			ElementTypeParams: map[string]string{
				"dim": "1024",
			},
		},
		{
			FieldName: "text",
			DataType:  varcharType,
			ElementTypeParams: map[string]string{
				"max_length": "65535",
			},
		},
		{
			FieldName: "title",
			DataType:  varcharType,
			ElementTypeParams: map[string]string{
				"max_length": "255",
			},
		},
		{
			FieldName: "user_email",
			DataType:  varcharType,
			ElementTypeParams: map[string]string{
				"max_length": "255",
			},
		},
	}

	indexParams := []*IndexParams{
		{
			MetricType: "COSINE",
			FieldName:  "vector",
			IndexName:  "vector_index",
			Params: map[string]string{
				"indexType": "HNSW",
			},
		},
		{
			FieldName: "user_email",
			IndexName: "user_email_index",
			Params: map[string]string{
				"indexType": "INVERTED",
			},
		},
	}

	createCollectionRequest := &CreateCollectionRequest{
		CollectionName: processor.DefaultCollectionName,
		Schema: &Schema{
			EnableDynamicField: false,
			Fields:             fields,
		},
		IndexParams: indexParams,
	}

	payload, err := json.Marshal(createCollectionRequest)
	if err != nil {
		slog.Error("Failed to marshal request", "err", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		slog.Error("Failed to create request", "err", err)
		return
	}

	req.Header.Add("Authorization", "Bearer "+config.Cfg.Milvus.APIKey)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("Failed to send request", "err", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	slog.Info("create milvus collection response", "body", string(body))
}
