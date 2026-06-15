package displayassets

import "embed"

// FS contains the built Svelte display assets. Release and Docker builds replace
// dist with the real web build before compiling juted.
//
//go:embed dist/*
var FS embed.FS
