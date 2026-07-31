//go:build cssstub

// Package metadatawriter implements a writer that generates Content-Range and Content-Length headers for range requests.
package metadatawriter

import "fmt"

type RangeMetadataWriter struct{}

func (w *RangeMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type

	size, _ := metadata["size"].(int)
	unit, _ := metadata["unit"].(string)
	if unit == "" {
		if size > 0 {
			setHeader(response, "Content-Length", itoa(size))
		}
		return nil
	}
	start, _ := metadata["start"].(int)
	end, hasEnd := metadata["end"].(int)
	if !hasEnd && size > 0 {
		end = size - 1
	}
	rangeHeader := unit + " " + itoaOrStar(start) + "-" + itoaOrStar(end) + "/" + itoaOrStar(size)
	setHeader(response, "Content-Range", rangeHeader)
	if start >= 0 && end >= 0 {
		setHeader(response, "Content-Length", itoa(end-start+1))
	}
	return nil
}

func itoaOrStar(val int) string {
	if val >= 0 {
		return itoa(val)
	}
	return "*"
}

func itoa(val int) string {
	return fmt.Sprintf("%d", val)
}
