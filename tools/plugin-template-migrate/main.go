package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type output struct {
	Command string   `json:"command"`
	Status  string   `json:"status"`
	Results []Result `json:"results"`
	Error   string   `json:"error,omitempty"`
}

func main() {
	os.Exit(runCLI(os.Args[1:]))
}

func runCLI(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}
	command := args[0]
	if command != "inspect" && command != "dry-run" && command != "apply" && command != "verify" {
		printUsage()
		return 2
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		writeOutput(output{Command: command, Status: "error", Error: err.Error()})
		return 1
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := Options{}
	flags.StringVar(&options.ManifestPath, "manifest", filepath.Join(workingDirectory, "backups", "plugin-template-migration-20260719", "manifest.json"), "迁移 manifest JSON 路径")
	flags.StringVar(&options.Root, "root", workingDirectory, "仓库根目录")
	flags.StringVar(&options.DBPath, "db", filepath.Join(workingDirectory, "config.db"), "SQLite 数据库路径")
	flags.StringVar(&options.Plugin, "plugin", "", "仅处理指定插件 ID")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		writeOutput(output{Command: command, Status: "error", Error: "存在未识别的位置参数"})
		return 2
	}
	results, err := newRunner().run(command, options)
	if err != nil {
		writeOutput(output{Command: command, Status: "error", Results: results, Error: err.Error()})
		return 1
	}
	writeOutput(output{Command: command, Status: "ok", Results: results})
	return 0
}

func writeOutput(value output) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "用法: go run ./tools/plugin-template-migrate <inspect|dry-run|apply|verify> [-manifest PATH] [-root PATH] [-db PATH] [-plugin ID]")
}
