//go:build !production

package logging

import (
	"io"
	"os"
)

// consoleWriter 在开发构建下把日志同时写到 stderr，保持终端可见的开发体验。
var consoleWriter io.Writer = os.Stderr
