package processor

import (
	"bytes"
	"context"
	"diabetes-agent-backend/model"
	knowledgebase "diabetes-agent-backend/service/knowledge-base"
	"fmt"
	"log/slog"

	"github.com/milvus-io/milvus/client/v2/column"
	client "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/textsplitter"
)

type PDFETLProcessor struct {
	BaseETLProcessor
}

var _ ETLProcessor = &PDFETLProcessor{}

func NewPDFETLProcessor() (*PDFETLProcessor, error) {
	separators := []string{"\n\n", "\n", "。", "！", "？", "；", "，", " ", ""}
	textSplitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators(separators),
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
	)

	baseETLProcessor, err := NewBaseETLProcessor(textSplitter)
	if err != nil {
		return nil, fmt.Errorf("error creating BaseETLProcessor: %v", err)
	}

	return &PDFETLProcessor{
		BaseETLProcessor: *baseETLProcessor,
	}, nil
}

func (p *PDFETLProcessor) CanProcess(fileType model.FileType) bool {
	return fileType == model.FileTypePDF
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

	slog.Debug("split pdf successfully",
		"object_name", objectName,
		"texts_num", len(texts),
	)

	// 生成文档切片的向量
	vectors, err := p.Embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return fmt.Errorf("error embedding pdf: %v", err)
	}

	slog.Debug("embedded pdf successfully",
		"object_name", objectName,
		"vectors_num", len(vectors),
	)

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
		return fmt.Errorf("error inserting pdf chunks: %v", err)
	}

	// 更新知识文件状态
	err = knowledgebase.UpdateKnowledgeMetadataStatus(objectName, model.StatusProcessed)
	if err != nil {
		return err
	}

	return nil
}
