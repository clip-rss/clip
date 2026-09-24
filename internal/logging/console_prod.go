//go:build production

package logging

import "io"

// consoleWriter 在生产构建下为 nil：GUI 应用的 stderr 不可见，只写文件。
var consoleWriter io.Writer = nil
