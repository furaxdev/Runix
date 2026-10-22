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
	// ListenHost is the host the daemon's HTTP server is actually bound
	// to (the host part of Config.ListenAddr). Used to refuse issuing a
	// pairing QR code that a phone could never reach: see
	// handlePairQRCode.
	ListenHost string
	IssueToken func(deviceName string) (token string, err error)
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
	// The QR code advertises a LAN address for the phone to connect to,
	// but if the daemon itself is only bound to loopback, nothing is
	// actually listening there: issuing the QR code anyway would leave
	// the phone stuck on a connection that can never succeed. Refuse
	// with a clear, actionable error instead (FR+EN, since the rest of
	// this project's user-facing errors are bilingual).
	if pairing.IsLoopbackHost(v.ListenHost) {
		writeError(w, http.StatusConflict, jsonErr(
			"pairing requires the daemon to listen on a LAN-reachable address; "+
				"set listen_addr to 0.0.0.0:<port> (or a specific LAN IP) in the config and restart / "+
				"l'appairage nécessite que le démon écoute sur une adresse accessible depuis le réseau local ; "+
				"réglez listen_addr sur 0.0.0.0:<port> (ou une IP locale précise) dans la config puis redémarrez"))
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
