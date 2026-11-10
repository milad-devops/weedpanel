package httpapi

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
)

// buildFilerUpload wraps raw file bytes into the multipart/form-data body that a
// SeaweedFS Filer requires for POST uploads (a raw-body POST is rejected with
// "request Content-Type isn't multipart/form-data"). contentType is the file's
// own MIME type; the Filer stores it and serves it back on download.
//
// The body is buffered so the request carries a Content-Length; callers are
// expected to pass a size-limited reader (http.MaxBytesReader).
func buildFilerUpload(name, contentType string, body io.Reader) (*bytes.Buffer, string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name}))
	header.Set("Content-Type", contentType)
	part, err := mw.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, body); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &buf, mw.FormDataContentType(), nil
}
