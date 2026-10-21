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

	binPath := resolveBackendBinary(cfg)
	if binPath == "" {
		return fmt.Errorf("no llama-server backend available; check your network connection and try again (see `runyx doctor`)")
	}

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

func chatLoop(baseURL, modelName string) error {
	var history []chatMsg
	scanner := bufio.NewScanner(os.Stdin)
	client := &http.Client{}

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
			"model":    modelName,
			"messages": history,
			"stream":   true,
		})
		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("chat request: %w", err)
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
