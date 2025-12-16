package processor

import (
	"bytes"
	"context"
	"diabetes-agent-backend/model"
	knowledgebase "diabetes-agent-backend/service/knowledge-base"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/milvus-io/milvus/client/v2/column"
	client "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/tmc/langchaingo/documentloaders"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
)

// MarkdownETLProcessor Markdown文件ETL处理器，兼容Text文件
type MarkdownETLProcessor struct {
	BaseETLProcessor
}

var _ ETLProcessor = &MarkdownETLProcessor{}

func NewMarkdownETLProcessor() (*MarkdownETLProcessor, error) {
	separators := []string{"\n\n", "\n", "。", "！", "？", "；", "，", " ", ""}
	textSplitter := textsplitter.NewMarkdownTextSplitter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
		textsplitter.WithHeadingHierarchy(true), // 保留父级标题信息
		textsplitter.WithSecondSplitter(textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(chunkSize),
			textsplitter.WithChunkOverlap(chunkOverlap),
			textsplitter.WithSeparators(separators),
		)),
	)

	baseETLProcessor, err := NewBaseETLProcessor(textSplitter)
	if err != nil {
		return nil, err
	}

	return &MarkdownETLProcessor{
		BaseETLProcessor: *baseETLProcessor,
	}, nil
}

func (p *MarkdownETLProcessor) CanProcess(fileType model.FileType) bool {
	return fileType == model.FileTypeMarkdown || fileType == model.FileTypeText
}

func (p *MarkdownETLProcessor) ExecuteETLPipeline(ctx context.Context, object []byte, objectName string) error {
	reader := bytes.NewReader(object)
	loader := documentloaders.NewText(reader)

	docs, err := loader.LoadAndSplit(ctx, p.TextSplitter)
	if err != nil {
		return fmt.Errorf("error loading and spliting markdown: %v", err)
	}

	// 过滤只有孤立标题的chunk
	docs, err = p.filterStandaloneHeaders(docs)
	if err != nil {
		return fmt.Errorf("error removing standalone headers: %v", err)
	}

	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.PageContent)
	}

	slog.Debug("split markdown successfully",
		"object_name", objectName,
		"texts_num", len(texts),
	)

	vectors, err := p.Embedder.EmbedDocuments(ctx, texts)
	if err != nil {
		return fmt.Errorf("error embedding markdown: %v", err)
	}

	slog.Debug("embedded markdown successfully",
		"object_name", objectName,
		"vectors_num", len(vectors),
	)

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
	_, err = p.MilvusClient.Insert(ctx, insertOption)
	if err != nil {
		return fmt.Errorf("error inserting markdown chunks: %v", err)
	}

	err = knowledgebase.UpdateKnowledgeMetadataStatus(objectName, model.StatusProcessed)
	if err != nil {
		return err
	}

	return nil
}

func (p *MarkdownETLProcessor) filterStandaloneHeaders(docs []schema.Document) ([]schema.Document, error) {
	var filteredDocs []schema.Document

	// 匹配形如 "# xxx ## xxx" 的chunk
	headerOnlyRegex := regexp.MustCompile(`^\s*(?:#{1,6}\s+.+\n?)+\s*$`)

	for _, doc := range docs {
		content := strings.TrimSpace(doc.PageContent)
		if content == "" {
			continue
		}

		if headerOnlyRegex.MatchString(content) {
			continue
		}

		filteredDocs = append(filteredDocs, doc)
	}
	return filteredDocs, nil
}
