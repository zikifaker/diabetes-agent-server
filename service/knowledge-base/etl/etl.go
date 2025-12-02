package etl

import (
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/service/knowledge-base/etl/processor"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// 知识文件ETL处理器注册表
var etlProcessorRegistry []processor.ETLProcessor

type ETLMessage struct {
	FileType   string `json:"file_type"`
	ObjectName string `json:"object_name"`
}

func init() {
	pdfProcessor, err := processor.NewPDFETLProcessor()
	if err != nil {
		panic(fmt.Sprintf("error creating PDFETLProcessor: %v", err))
	}

	etlProcessorRegistry = []processor.ETLProcessor{
		pdfProcessor,
	}
}

func HandleETLMessage(ctx context.Context, msg *primitive.MessageExt) error {
	var etlMessage ETLMessage
	if err := json.Unmarshal(msg.Body, &etlMessage); err != nil {
		return fmt.Errorf("failed to unmarshal message body: %v", err)
	}

	object, err := getObjectFromOSS(ctx, &etlMessage)
	if err != nil {
		return fmt.Errorf("failed to get object from oss: %v", err)
	}

	// 查找匹配文件类型的处理器，执行ETL流程
	foundProcessor := false
	for _, processor := range etlProcessorRegistry {
		if processor.CanProcess(etlMessage.FileType) {
			foundProcessor = true
			if err := processor.ExecuteETLPipeline(ctx, object, etlMessage.ObjectName); err != nil {
				return fmt.Errorf("failed to execute ETL pipeline: %v", err)
			}
			return nil
		}
	}

	if !foundProcessor {
		return fmt.Errorf("no processor found for file type: %s", etlMessage.FileType)
	}

	return nil
}

func getObjectFromOSS(ctx context.Context, etlMessage *ETLMessage) ([]byte, error) {
	cfg := &oss.Config{
		Region: oss.Ptr(config.Cfg.OSS.Region),
		CredentialsProvider: credentials.NewStaticCredentialsProvider(
			config.Cfg.OSS.AccessKeyID,
			config.Cfg.OSS.AccessKeySecret,
		),
	}
	client := oss.NewClient(cfg)

	result, err := client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(config.Cfg.OSS.BucketName),
		Key:    oss.Ptr(etlMessage.ObjectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from oss: %v", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %v", err)
	}

	return data, nil
}
