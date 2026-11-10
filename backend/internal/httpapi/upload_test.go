package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type receivedUpload struct {
	field       string
	filename    string
	contentType string
	data        []byte
}

// fakeFiler mimics the SeaweedFS Filer's POST behaviour: only multipart/form-data
// bodies are accepted; anything else is rejected with Go's ErrNotMultipart text.
func fakeFiler(got *receivedUpload) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		data, _ := io.ReadAll(part)
		*got = receivedUpload{part.FormName(), part.FileName(), part.Header.Get("Content-Type"), data}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"f","size":` + strconv.Itoa(len(data)) + `}`))
	}))
}

func binaryPayload() []byte {
	b := make([]byte, 0, 256*40)
	for i := 0; i < 40; i++ {
		for v := 0; v < 256; v++ {
			b = append(b, byte(v))
		}
	}
	return b
}

// Reproduces the reported bug: a raw-body POST (what the panel used to send) is rejected.
func TestRawPostIsRejectedByFiler(t *testing.T) {
	var got receivedUpload
	srv := fakeFiler(&got)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/photos/a.jpg", "image/jpeg", bytes.NewReader(binaryPayload()))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "multipart/form-data") {
		t.Fatalf("expected the not-multipart rejection, got %d %s", resp.StatusCode, body)
	}
}

func TestBuildFilerUploadIsAcceptedByFiler(t *testing.T) {
	names := []string{"photo.jpg", `my "quoted" file.png`, "عکس تست.jpg", "with space & symbols.jpg"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			var got receivedUpload
			srv := fakeFiler(&got)
			defer srv.Close()

			payload := binaryPayload()
			body, contentType, err := buildFilerUpload(name, "image/jpeg", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequest(http.MethodPost, srv.URL+"/photos/x", body)
			req.Header.Set("Content-Type", contentType)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("filer rejected upload: %d %s", resp.StatusCode, b)
			}
			if got.field != "file" {
				t.Errorf("form field = %q, want file", got.field)
			}
			if got.filename != name {
				t.Errorf("filename = %q, want %q", got.filename, name)
			}
			if got.contentType != "image/jpeg" {
				t.Errorf("part content-type = %q, want image/jpeg", got.contentType)
			}
			if !bytes.Equal(got.data, payload) {
				t.Errorf("payload changed in transit (%d bytes in, %d out)", len(payload), len(got.data))
			}
		})
	}
}

func TestBuildFilerUploadDefaultsContentType(t *testing.T) {
	var got receivedUpload
	srv := fakeFiler(&got)
	defer srv.Close()

	body, contentType, err := buildFilerUpload("blob", "", strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/x", body)
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got.contentType != "application/octet-stream" {
		t.Errorf("part content-type = %q, want application/octet-stream", got.contentType)
	}
}

func TestBuildFilerUploadReportsSizeLimit(t *testing.T) {
	limited := http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(bytes.NewReader(make([]byte, 100))), 10)
	_, _, err := buildFilerUpload("big.bin", "", limited)
	var tooLarge *http.MaxBytesError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("expected *http.MaxBytesError, got %v", err)
	}
}
