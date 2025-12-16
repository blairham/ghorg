package cmd

import (
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/blairham/ghorg/internal/colorlog"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

type RecloneServerCommand struct {
	UI cli.Ui
}

type RecloneServerFlags struct {
	Port string `short:"p" long:"port" description:"GHORG_RECLONE_SERVER_PORT - Specify the port the reclone server will run on"`
}

func (c *RecloneServerCommand) Help() string {
	return `Usage: ghorg reclone-server [options]

Server allowing you to trigger ad hoc reclone commands via HTTP requests.

Options:
  -p, --port    Port for the reclone server

Examples:
  ghorg reclone-server --port 8080
  ghorg reclone-server -p 9000

Endpoints:
  /trigger/reclone?cmd=<reclone-key>   Trigger a reclone
  /stats                                View stats (requires GHORG_STATS_ENABLED=true)
  /health                               Health check

Read the documentation and examples in the Readme under Reclone Server heading.
`
}

func (c *RecloneServerCommand) Synopsis() string {
	return "Server allowing ad hoc reclone commands via HTTP"
}

func (c *RecloneServerCommand) Run(args []string) int {
	var opts RecloneServerFlags
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.ParseArgs(args)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			fmt.Println(c.Help())
			return 0
		}
		colorlog.PrintError(fmt.Sprintf("Error parsing flags: %v", err))
		return 1
	}

	if opts.Port != "" {
		os.Setenv("GHORG_RECLONE_SERVER_PORT", opts.Port)
	}

	startReCloneServer()
	return 0
}

func startReCloneServer() {
	var mu sync.Mutex
	serverPort := os.Getenv("GHORG_RECLONE_SERVER_PORT")
	if serverPort != "" && serverPort[0] != ':' {
		serverPort = ":" + serverPort
	}

	http.HandleFunc("/trigger/reclone", func(w http.ResponseWriter, r *http.Request) {
		userCmd := r.URL.Query().Get("cmd")

		if !mu.TryLock() {
			http.Error(w, "Server is busy, please try again later", http.StatusTooManyRequests)
			return
		}

		// Signal channel to notify when the command has started
		started := make(chan struct{})

		go func() {
			defer mu.Unlock()
			var cmd *exec.Cmd
			if userCmd == "" {
				cmd = exec.Command("ghorg", "reclone")
			} else {
				cmd = exec.Command("ghorg", "reclone", userCmd)
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Notify that the command has started
			close(started)

			if err := cmd.Run(); err != nil {
				fmt.Printf("Error running command: %s\n", err)
			}
		}()

		// Wait for the command to start before responding
		<-started
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {

		if os.Getenv("GHORG_STATS_ENABLED") != "true" {
			http.Error(w, "Stats collection is not enabled. Please set GHORG_STATS_ENABLED=true or use --stats-enabled flag", http.StatusPreconditionRequired)
			return
		}

		statsFilePath := getGhorgStatsFilePath()
		fileExists := true

		if _, err := os.Stat(statsFilePath); os.IsNotExist(err) {
			fileExists = false
		}

		if fileExists {
			file, err := os.Open(statsFilePath)
			if err != nil {
				http.Error(w, "Unable to open file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			reader := csv.NewReader(file)
			records, err := reader.ReadAll()
			if err != nil {
				http.Error(w, "Unable to read CSV file", http.StatusInternalServerError)
				return
			}

			var jsonData []map[string]string
			headers := records[0]
			for _, row := range records[1:] {
				rowData := make(map[string]string)
				for i, value := range row {
					rowData[headers[i]] = value
				}
				jsonData = append(jsonData, rowData)
			}

			jsonBytes, err := json.Marshal(jsonData)
			if err != nil {
				http.Error(w, "Unable to encode JSON", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBytes)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	colorlog.PrintInfo("Starting reclone server on " + serverPort)
	if err := http.ListenAndServe(serverPort, nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
