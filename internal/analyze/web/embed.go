// Package web embeds the static frontend for evalctl analyze
// (research.md §5 — no Node/build step, plain HTML/CSS/JS).
package web

import "embed"

//go:embed index.html app.js style.css
var FS embed.FS
