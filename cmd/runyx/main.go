// Command runyx is the Runyx CLI, daemon, and HTTP server, all in one
// binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error

	switch cmd {
	case "version":
		err = cmdVersion(args)
	case "serve":
		err = cmdServe(args)
	case "doctor":
		err = cmdDoctor(args)
	case "recommend":
		err = cmdRecommend(args)
	case "tune":
		err = cmdTune(args)
	case "pull":
		err = cmdPull(args)
	case "run":
		err = cmdRun(args)
	case "list", "ls":
		err = cmdList(args)
	case "rm":
		err = cmdRemove(args)
	case "show":
		err = cmdShow(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "runyx: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "runyx: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `runyx — a local LLM runner

Usage:
  runyx version              Print version info
  runyx serve                Run the daemon in the foreground
  runyx doctor                Detect CPU/RAM/GPU and backend availability
  runyx recommend              Suggest models that fit this machine
  runyx tune <model>           Compute and cache tuning params for a model
  runyx pull <repo>[:file]      Download a GGUF model from Hugging Face
  runyx run <model>            Chat with a model (pulling if needed)
  runyx list                  List installed models
  runyx show <model>           Show details about an installed model
  runyx rm <model>             Remove an installed model
`)
}
