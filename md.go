package main

import (
	"bytes"

	"io"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func markdownParseBytes(source []byte) bytes.Buffer {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	var buffer bytes.Buffer
	if err := md.Convert(source, &buffer); err != nil {
		panic(err)
	}
	return buffer
}

func MarkdownParse(source string) string {
	buffer := markdownParseBytes([]byte(source))
	return buffer.String()
}

func MarkdownParseFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	buffer := markdownParseBytes(fileBytes)
	return buffer.String(), nil
}
