package web

import "embed"

//go:embed static/*.css static/*.js templates/*.html
var assets embed.FS
