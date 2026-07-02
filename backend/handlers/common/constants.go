package common

const (
	MaxImageSize  = 8 << 20   // 8 MB
	MaxBinarySize = 512 << 20 // 512 MB
	MaxJSONSize   = 1 << 20   // 1 MB
)

// Entity type constants for entity_translations table.
const (
	EntityTypeApp          = "app"
	EntityTypeProjectImage = "project_image"
	EntityTypeVersion      = "version"
	EntityTypeProject      = "project"
)
