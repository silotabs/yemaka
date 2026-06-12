package rag

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"yemaka/internal/safety"
)

const (
	mimePDF              = "application/pdf"
	mimeDOCX             = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	maxExtractedPDFPages = 100
)

type extractedDocument struct {
	Path     string
	Content  string
	Size     int64
	MimeType string
}

func extractDocument(ctx context.Context, root string, rel string, maxBytes int64) (extractedDocument, error) {
	select {
	case <-ctx.Done():
		return extractedDocument{}, ctx.Err()
	default:
	}
	abs, cleanRel, err := safety.ResolveReadPath(safety.PathPolicy{WorkspaceRoot: root}, rel)
	if err != nil {
		return extractedDocument{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return extractedDocument{}, fmt.Errorf("stat document: %w", err)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return extractedDocument{}, fmt.Errorf("document exceeds max read size: %s", cleanRel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return extractedDocument{}, fmt.Errorf("read document: %w", err)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return extractedDocument{}, fmt.Errorf("document exceeds max read size: %s", cleanRel)
	}

	ext := strings.ToLower(filepath.Ext(cleanRel))
	var text string
	var mimeType string
	switch ext {
	case ".docx":
		text, err = extractDOCXText(data)
		mimeType = mimeDOCX
	case ".pdf":
		text, err = extractPDFText(data)
		mimeType = mimePDF
	default:
		return extractedDocument{}, fmt.Errorf("unsupported extractable document: %s; supported document types: .pdf, .docx", cleanRel)
	}
	if err != nil {
		return extractedDocument{}, err
	}
	text = normalizeExtractedText(text)
	if text == "" {
		return extractedDocument{}, fmt.Errorf("no text extracted from %s", cleanRel)
	}
	return extractedDocument{Path: cleanRel, Content: text, Size: info.Size(), MimeType: mimeType}, nil
}

func isExtractableDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf", ".docx":
		return true
	default:
		return false
	}
}

func extractDOCXText(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx container: %w", err)
	}
	var builder strings.Builder
	for _, file := range reader.File {
		if !isWordDocumentPart(file.Name) {
			continue
		}
		text, err := extractWordXML(file)
		if err != nil {
			return "", err
		}
		if text != "" {
			builder.WriteString(text)
			builder.WriteString("\n")
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", fmt.Errorf("docx contains no readable text")
	}
	return text, nil
}

func isWordDocumentPart(name string) bool {
	name = filepath.ToSlash(name)
	return name == "word/document.xml" ||
		strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") ||
		strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml") ||
		name == "word/footnotes.xml" ||
		name == "word/endnotes.xml" ||
		name == "word/comments.xml"
}

func extractWordXML(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open docx xml %s: %w", file.Name, err)
	}
	defer reader.Close()

	decoder := xml.NewDecoder(reader)
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("parse docx xml %s: %w", file.Name, err)
		}
		switch value := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if text != "" {
				if needsSpace(builder.String()) {
					builder.WriteByte(' ')
				}
				builder.WriteString(text)
			}
		case xml.StartElement:
			switch value.Name.Local {
			case "tab":
				builder.WriteByte('\t')
			case "br":
				builder.WriteByte('\n')
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "tc":
				builder.WriteByte('\t')
			case "tr", "p":
				builder.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

func extractPDFText(data []byte) (string, error) {
	if pages := countPDFPages(data); pages > maxExtractedPDFPages {
		return "", fmt.Errorf("pdf page count %d exceeds local extraction limit %d", pages, maxExtractedPDFPages)
	}
	var builder strings.Builder
	streams := pdfStreams(data)
	for _, stream := range streams {
		appendPDFStrings(&builder, string(stream))
	}
	if builder.Len() == 0 {
		appendPDFStrings(&builder, string(data))
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", fmt.Errorf("pdf contains no simple extractable text")
	}
	return text, nil
}

func countPDFPages(data []byte) int {
	count := 0
	offset := 0
	for {
		index := bytes.Index(data[offset:], []byte("/Type"))
		if index < 0 {
			return count
		}
		index += offset + len("/Type")
		index = skipPDFWhitespace(data, index)
		if bytes.HasPrefix(data[index:], []byte("/Page")) && !bytes.HasPrefix(data[index:], []byte("/Pages")) {
			count++
		}
		offset = index
	}
}

func skipPDFWhitespace(data []byte, index int) int {
	for index < len(data) {
		switch data[index] {
		case ' ', '\n', '\r', '\t', '\f', 0:
			index++
		default:
			return index
		}
	}
	return index
}

func pdfStreams(data []byte) [][]byte {
	var streams [][]byte
	offset := 0
	for {
		start := bytes.Index(data[offset:], []byte("stream"))
		if start < 0 {
			break
		}
		start += offset
		bodyStart := start + len("stream")
		if bodyStart < len(data) && data[bodyStart] == '\r' {
			bodyStart++
		}
		if bodyStart < len(data) && data[bodyStart] == '\n' {
			bodyStart++
		}
		end := bytes.Index(data[bodyStart:], []byte("endstream"))
		if end < 0 {
			break
		}
		bodyEnd := bodyStart + end
		body := bytes.TrimRight(data[bodyStart:bodyEnd], "\r\n")
		dictStart := start - 1024
		if dictStart < 0 {
			dictStart = 0
		}
		dict := data[dictStart:start]
		if bytes.Contains(dict, []byte("/FlateDecode")) {
			if decoded, err := inflatePDFStream(body); err == nil {
				body = decoded
			}
		}
		streams = append(streams, body)
		offset = bodyEnd + len("endstream")
	}
	return streams
}

func inflatePDFStream(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func appendPDFStrings(builder *strings.Builder, input string) {
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '(':
			value, next, ok := parsePDFLiteral(input, i)
			if ok {
				appendExtractedValue(builder, value)
				i = next
			}
		case '<':
			if i+1 < len(input) && input[i+1] == '<' {
				continue
			}
			value, next, ok := parsePDFHexString(input, i)
			if ok {
				appendExtractedValue(builder, value)
				i = next
			}
		}
	}
}

func parsePDFLiteral(input string, start int) (string, int, bool) {
	var out []byte
	depth := 1
	for i := start + 1; i < len(input); i++ {
		ch := input[i]
		if ch == '\\' {
			if i+1 >= len(input) {
				return "", i, false
			}
			next := input[i+1]
			switch next {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, next)
			case '\n':
			case '\r':
				if i+2 < len(input) && input[i+2] == '\n' {
					i++
				}
			default:
				if next >= '0' && next <= '7' {
					end := i + 2
					for end+1 < len(input) && end-i < 4 && input[end+1] >= '0' && input[end+1] <= '7' {
						end++
					}
					value, err := strconv.ParseInt(input[i+1:end+1], 8, 32)
					if err == nil {
						out = append(out, byte(value))
					}
					i = end
					continue
				}
				out = append(out, next)
			}
			i++
			continue
		}
		switch ch {
		case '(':
			depth++
			out = append(out, ch)
		case ')':
			depth--
			if depth == 0 {
				return decodeTextBytes(out), i, true
			}
			out = append(out, ch)
		default:
			out = append(out, ch)
		}
	}
	return "", len(input), false
}

func parsePDFHexString(input string, start int) (string, int, bool) {
	end := strings.IndexByte(input[start+1:], '>')
	if end < 0 {
		return "", start, false
	}
	end += start + 1
	raw := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' || r == '\f' {
			return -1
		}
		return r
	}, input[start+1:end])
	if raw == "" || len(raw) > 4096 {
		return "", end, false
	}
	if len(raw)%2 == 1 {
		raw += "0"
	}
	data, err := hex.DecodeString(raw)
	if err != nil {
		return "", end, false
	}
	return decodeTextBytes(data), end, true
}

func appendExtractedValue(builder *strings.Builder, value string) {
	value = normalizeExtractedText(value)
	if value == "" || !hasLetterOrDigit(value) {
		return
	}
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString(value)
}

func decodeTextBytes(data []byte) string {
	if len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return string(utf16.Decode(units))
	}
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		}
		return string(utf16.Decode(units))
	}
	if utf8.Valid(data) {
		return string(data)
	}
	runes := make([]rune, 0, len(data))
	for _, b := range data {
		runes = append(runes, rune(b))
	}
	return string(runes)
}

func normalizeExtractedText(input string) string {
	lines := strings.Split(input, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func hasLetterOrDigit(input string) bool {
	for _, r := range input {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func needsSpace(input string) bool {
	if input == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(input)
	return last != '\n' && last != '\t' && last != ' '
}
