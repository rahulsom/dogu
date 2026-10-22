package dogu

import _ "embed"

// DefaultConfigBytes exports the raw contents of default config.json.
//go:embed config.json
var DefaultConfigBytes []byte
