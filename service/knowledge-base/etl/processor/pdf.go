package processor

import (
	"bytes"
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/model"
	"diabetes-agent-backend/service/chat"
	knowledgebase "diabetes-agent-backend/service/knowledge-base"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	client "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/textsplitter"
)

const (
	embeddingModelName = "text-embedding-v4"
	chunkSize          = 4000
	chunkOverlap       = 200
	embeddingBatchSize = 10
	vectorDim          = 1024

	CollectionName = "knowledge_doc"
)

type PDFETLProcessor struct {
	TextSplitter textsplitter.TextSplitter
	Embedder     embeddings.Embedder
	MilvusClient *milvusclient.Client
}

var _ ETLProcessor = &PDFETLProcessor{}

func NewPDFETLProcessor() (*PDFETLProcessor, error) {
	textSplitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators([]string{"\n\n", "\n", "。", "！", "？", "；", "，", " "}),
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
	)

	client, err := openai.New(
		openai.WithEmbeddingModel(embeddingModelName),
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
		embeddings.WithBatchSize(embeddingBatchSize),
		embeddings.WithStripNewLines(false),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %v", err)
	}

	milvusConfig := milvusclient.ClientConfig{
		Address: config.Cfg.Milvus.Endpoint,
		APIKey:  config.Cfg.Milvus.APIKey,
	}

	milvusClient, err := milvusclient.New(context.Background(), &milvusConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus client: %v", err)
	}

	return &PDFETLProcessor{
		TextSplitter: textSplitter,
		Embedder:     embedder,
		MilvusClient: milvusClient,
	}, nil
}

func (p *PDFETLProcessor) CanProcess(fileType string) bool {
	return fileType == "pdf"
}

func (p *PDFETLProcessor) ExecuteETLPipeline(ctx context.Context, data []byte, objectName string) error {
	reader := bytes.NewReader(data)
	loader := documentloaders.NewPDF(reader, int64(len(data)))

	// 切分文档
	docs, err := loader.LoadAndSplit(ctx, p.TextSplitter)
	if err != nil {
		return fmt.Errorf("error loading and spliting pdf: %v", err)
	}

	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.PageContent)
	}

	slog.Debug("split documents successfully", "texts_num", len(texts))

	// 生成文档切片的向量
	vectors, err := p.Embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return fmt.Errorf("error embedding documents: %v", err)
	}

	slog.Debug("embedded documents successfully", "vectors_num", len(vectors))

	// 组装列数据，包括文档切片、向量和元数据
	columns := make([]column.Column, 0)
	columns = append(columns, column.NewColumnVarChar("text", texts))
	columns = append(columns, column.NewColumnFloatVector("vector", vectorDim, vectors))

	columns, err = addMetadataColumns(columns, len(texts), &Metadata{
		objectName: objectName,
	})
	if err != nil {
		return fmt.Errorf("error adding metadata columns: %v", err)
	}

	insertOption := client.NewColumnBasedInsertOption(CollectionName).WithColumns(columns...)

	// 加载数据到milvus
	_, err = p.MilvusClient.Insert(ctx, insertOption)
	if err != nil {
		return fmt.Errorf("error inserting document chunks: %v", err)
	}

	// 更新知识文件状态
	err = knowledgebase.UpdateKnowledgeMetadataStatus(objectName, model.StatusProcessed)
	if err != nil {
		return err
	}

	return nil
}

func (p *PDFETLProcessor) DeleteVectorStore(ctx context.Context, objectName string) error {
	pathSegments := strings.Split(objectName, "/")
	if len(pathSegments) < 2 {
		return fmt.Errorf("invalid object name: %s", objectName)
	}

	userEmail := pathSegments[0]
	fileName := pathSegments[len(pathSegments)-1]

	expression := fmt.Sprintf("user_email == '%s' and title == '%s'", userEmail, fileName)
	deleteOption := milvusclient.NewDeleteOption(CollectionName).WithExpr(expression)

	_, err := p.MilvusClient.Delete(ctx, deleteOption)
	if err != nil {
		return fmt.Errorf("error deleting document chunks: %v", err)
	}

	return nil
}
