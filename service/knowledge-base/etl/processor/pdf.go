package processor

import (
	"bytes"
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/service/chat"
	"fmt"
	"net/http"
	"time"

	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/textsplitter"
	"github.com/tmc/langchaingo/vectorstores"
	v2 "github.com/tmc/langchaingo/vectorstores/milvus/v2"
)

const (
	defaultEmbeddingModel     = "text-embedding-v4"
	defaultChunkSize          = 4000
	defaultChunkOverlap       = 200
	defaultEmbeddingBatchSize = 10

	DefaultCollectionName = "knowledge_doc"
)

type PDFETLProcessor struct {
	TextSplitter textsplitter.TextSplitter
	Embedder     embeddings.Embedder
	VectorStore  vectorstores.VectorStore
}

var _ ETLProcessor = &PDFETLProcessor{}

func NewPDFETLProcessor() (*PDFETLProcessor, error) {
	textSplitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators([]string{"\n\n", "\n", "。", "！", "？", "；", "，", " ", ""}),
		textsplitter.WithChunkSize(defaultChunkSize),
		textsplitter.WithChunkOverlap(defaultChunkOverlap),
	)

	client, err := openai.New(
		openai.WithEmbeddingModel(defaultEmbeddingModel),
		openai.WithToken(config.Cfg.Model.APIKey),
		openai.WithBaseURL(chat.BaseURL),
		openai.WithHTTPClient(&http.Client{
			Timeout: 60 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder client: %v", err)
	}

	embedder, err := embeddings.NewEmbedder(client,
		embeddings.WithBatchSize(defaultEmbeddingBatchSize),
		embeddings.WithStripNewLines(false),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %v", err)
	}

	config := milvusclient.ClientConfig{
		Address: config.Cfg.Milvus.Endpoint,
		APIKey:  config.Cfg.Milvus.APIKey,
	}

	store, err := v2.New(context.Background(), config,
		v2.WithEmbedder(embedder),
		v2.WithCollectionName(DefaultCollectionName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus client: %v", err)
	}

	return &PDFETLProcessor{
		TextSplitter: textSplitter,
		Embedder:     embedder,
		VectorStore:  store,
	}, nil
}

func (p *PDFETLProcessor) CanProcess(fileType string) bool {
	return fileType == "pdf"
}

func (p *PDFETLProcessor) ExecuteETLPipeline(ctx context.Context, data []byte) error {
	reader := bytes.NewReader(data)
	loader := documentloaders.NewPDF(reader, int64(len(data)))

	docs, err := loader.LoadAndSplit(ctx, p.TextSplitter)
	if err != nil {
		return fmt.Errorf("error loading and spliting pdf: %v", err)
	}

	_, err = p.VectorStore.AddDocuments(ctx, docs)
	if err != nil {
		return fmt.Errorf("error loading documents to vector store: %v", err)
	}

	return nil
}
