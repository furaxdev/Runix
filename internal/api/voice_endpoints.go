package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/furaxdev/runyx/internal/convert"
	"github.com/furaxdev/runyx/internal/pairing"
	"github.com/furaxdev/runyx/internal/voice"
)

func decodeJSONBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// VoiceExtras wires the voice pipeline (item 25) and Android pairing
// (item 26) into the HTTP API.
type VoiceExtras struct {
	Transcriber *voice.Transcriber // nil disables /voice/transcribe
	Synthesizer *voice.Synthesizer // nil disables /voice/synthesize
	PairHost    string
	PairPort    int
	IssueToken  func(deviceName string) (token string, err error)
}

// RegisterVoice wires voice + pairing routes onto mux.
func (v *VoiceExtras) RegisterVoice(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/runyx/voice/transcribe", v.handleTranscribe)
	mux.HandleFunc("POST /api/runyx/voice/synthesize", v.handleSynthesize)
	mux.HandleFunc("POST /api/runyx/pairing/qrcode", v.handlePairQRCode)
}

func (v *VoiceExtras) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if v.Transcriber == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("speech-to-text is not configured (whisper.cpp not found)"))
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "runyx-voice-*-"+sanitizeBase(header.Filename))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer os.Remove(tmp.Name())
	io.Copy(tmp, file)
	tmp.Close()

	res, err := convert.AudioToText(r.Context(), v.Transcriber, tmp.Name())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": res.Text, "warnings": res.Warnings})
}

func (v *VoiceExtras) handleSynthesize(w http.ResponseWriter, r *http.Request) {
	if v.Synthesizer == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("text-to-speech is not configured (Piper not found)"))
		return
	}
	var req struct{ Text string }
	if decodeErr := decodeJSONBody(r, &req); decodeErr != nil {
		writeError(w, http.StatusBadRequest, decodeErr)
		return
	}
	tmp, err := os.CreateTemp("", "runyx-tts-*.wav")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	if err := v.Synthesizer.Synthesize(r.Context(), req.Text, tmp.Name()); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Write(data)
}

func (v *VoiceExtras) handlePairQRCode(w http.ResponseWriter, r *http.Request) {
	if v.IssueToken == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("pairing is not configured"))
		return
	}
	var req struct{ DeviceName string }
	decodeJSONBody(r, &req)
	if req.DeviceName == "" {
		req.DeviceName = "android"
	}
	token, err := v.IssueToken(req.DeviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	host := v.PairHost
	if host == "" {
		host = pairing.LocalIPv4()
	}
	info := pairing.Info{Host: host, Port: v.PairPort, Token: token}
	png, err := info.QRCodePNG(256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
