package roots

import "embed"

//go:embed tuf-root
var EmbeddedTUF embed.FS
