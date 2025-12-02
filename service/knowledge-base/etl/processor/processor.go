package processor

import (
	"context"
)

// ETLProcessor 知识文件ETL处理器
type ETLProcessor interface {
	// 判断是否支持传入的文件类型
	CanProcess(fileType string) bool

	// 执行ETL流程
	ExecuteETLPipeline(ctx context.Context, object []byte, objectName string) error
}
