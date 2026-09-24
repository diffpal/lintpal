package schema

import _ "embed"

// FindingsV5 is the pinned shared findings schema used for offline validation.
//
//go:embed findings-v5.schema.json
var FindingsV5 []byte
