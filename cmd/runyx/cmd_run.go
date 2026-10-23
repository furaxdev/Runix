package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// cmdRun pulls the model if needed, loads it via an in-process backend
// manager, and drops into an interactive chat loop against its
// llama-server instance.
func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx run <model>")
	}
	name := args[0]

	cfg, store, err := loadStore()
	if err != nil {
		return err
	}

	if _, ok, _ := store.Get(name); !ok {
		fmt.Printf("Model %q not installed yet, pulling...\n", name)
		if err := cmdPull([]string{name}); err != nil {
			return fmt.Errorf("auto-pull %q: %w", name, err)
		}
	}

	binPath, err := resolveBackendBinaryBlocking(cfg, func(p downloader.Progress) {
		printProgress(p)
	})
	if err != nil {
		fmt.Println()
		return fmt.Errorf("fetch llama-server backend: %w", err)
	}
	fmt.Println()

	mdl, ok, err := store.Get(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model %q not found after pull", name)
	}

	if mdl.Tuning == (modelstore.TuningParams{}) {
		info := hwdetect.Detect()
		t := hwdetect.Tune(info, int(mdl.SizeBytes/(1024*1024)))
		mdl.Tuning = modelstore.TuningParams{
			NGPULayers: t.NGPULayers, Threads: t.Threads,
			CtxSize: t.CtxSize, BatchSize: t.BatchSize, FlashAttn: t.FlashAttn,
		}
		_ = store.Put(mdl)
	}

	mgr := backend.NewManager(binPath, 0, cfg.VRAMBudgetMB) // no idle eviction mid-chat
	defer mgr.StopAll()

	fmt.Printf("Loading %s...\n", mdl.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inst, err := mgr.Start(ctx, mdl, mdl.Tuning)
	if err != nil {
		return fmt.Errorf("start model: %w", err)
	}
	fmt.Printf("Ready. Chatting with %s. Type /exit to quit.\n\n", mdl.Name)

	return chatLoop(inst.BaseURL, mdl.Name)
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequestTimeout bounds one full chat exchange (request plus
// reading the entire streamed reply). Without it a runaway generation
// — the model never producing a stop token, which does happen in
// practice, not just hypothetically — hangs this CLI forever with no
// way to recover short of killing the process.
const chatRequestTimeout = 3 * time.Minute

// chatMaxTokens caps how many tokens a single reply may generate, as a
// second line of defense alongside chatRequestTimeout: it turns a
// still-slow-but-not-yet-timed-out runaway generation into a bounded,
// visibly truncated reply instead of 3 minutes of churn.
const chatMaxTokens = 1024

func chatLoop(baseURL, modelName string) error {
	var history []chatMsg
	scanner := bufio.NewScanner(os.Stdin)
	client := &http.Client{Timeout: chatRequestTimeout}

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		history = append(history, chatMsg{Role: "user", Content: line})

		body, _ := json.Marshal(map[string]any{
			"model":      modelName,
			"messages":   history,
			"stream":     true,
			"max_tokens": chatMaxTokens,
		})
		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("chat request (no reply within %s — the model may be stuck generating): %w", chatRequestTimeout, err)
		}

		fmt.Print("runyx> ")
		var reply strings.Builder
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			l := sc.Text()
			if !strings.HasPrefix(l, "data: ") {
				continue
			}
			data := strings.TrimPrefix(l, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
				fmt.Print(chunk.Choices[0].Delta.Content)
				reply.WriteString(chunk.Choices[0].Delta.Content)
			}
		}
		resp.Body.Close()
		fmt.Println()
		fmt.Println()
		history = append(history, chatMsg{Role: "assistant", Content: reply.String()})
	}
}
