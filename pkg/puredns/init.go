package puredns

import (
	"io"

	log "github.com/sirupsen/logrus"
	_ "github.com/zmap/zdns/pkg/zdns"
)

func init() {
	// 默认禁用日志输出
	log.SetOutput(io.Discard)
}
