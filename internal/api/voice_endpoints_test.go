package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribeDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "clip.wav")
	part.Write([]byte("fake"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/runyx/voice/transcribe", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestSynthesizeDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/voice/synthesize", map[string]string{"Text": "bonjour"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestPairingQRCodeIssuesTokenAndReturnsPNG(t *testing.T) {
	mux := http.NewServeMux()
	var issuedFor string
	v := &VoiceExtras{
		PairHost: "192.168.1.10",
		PairPort: 11535,
		IssueToken: func(name string) (string, error) {
			issuedFor = name
			return "runyx_test_token", nil
		},
	}
	v.RegisterVoice(mux)

	body, _ := json.Marshal(map[string]string{"DeviceName": "mon-telephone"})
	req := httptest.NewRequest("POST", "/api/runyx/pairing/qrcode", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Error("response body doesn't look like a PNG")
	}
	if issuedFor != "mon-telephone" {
		t.Errorf("IssueToken called with %q, want mon-telephone", issuedFor)
	}
}

func TestPairingDisabledWithoutIssueToken(t *testing.T) {
	mux := http.NewServeMux()
	v := &VoiceExtras{}
	v.RegisterVoice(mux)
	rec := doJSON(t, mux, "POST", "/api/runyx/pairing/qrcode", map[string]string{})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}
