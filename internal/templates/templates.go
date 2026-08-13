// Package templates embeds the files phpflash scaffolds into a new project. It lives
// in its own package because go:embed can only read files inside the embedding file's
// own directory tree (no ".." traversal), and both config and project need these.
package templates

import _ "embed"

//go:embed config.toml.tmpl
var Config string

//go:embed ext.c.tmpl
var ExtC string

//go:embed index.hello.php
var IndexHello []byte

//go:embed index.blink.php
var IndexBlink []byte

//go:embed gitignore
var Gitignore []byte
