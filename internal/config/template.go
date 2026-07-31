package config

import (
	"bytes"
	"text/template"

	"phpflash/internal/templates"
)

func (c *Config) render() ([]byte, error) {
	t, err := template.New("config").Parse(templates.Config)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
