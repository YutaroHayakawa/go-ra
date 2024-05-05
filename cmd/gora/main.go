package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/YutaroHayakawa/go-ra"
	"github.com/YutaroHayakawa/go-ra/cmd/internal"
	"gopkg.in/yaml.v3"
)

func usageRoot() {
	fmt.Printf("Usage: %s <subcommand> [options]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  reload\tReload the configuration")
	fmt.Println("  status\tGet the status of the service")
	fmt.Println("  help\t\tShow this message")
}

func main() {
	if len(os.Args) < 2 {
		usageRoot()
		os.Exit(1)
	}

	if os.Args[1] == "help" {
		usageRoot()
		os.Exit(0)
	}

	client := internal.NewClient("localhost:8888")

	if os.Args[1] == "reload" {
		var (
			config string
		)
		command := flag.NewFlagSet("reload", flag.ExitOnError)
		command.StringVar(&config, "f", "", "config file path")
		command.Parse(os.Args[2:])
		reload(client, config)
	}

	if os.Args[1] == "status" {
		var (
			output string
		)
		command := flag.NewFlagSet("status", flag.ExitOnError)
		command.StringVar(&output, "o", "table", "Output format (table, json, or yaml)")
		command.Parse(os.Args[2:])
		status(client, output)
	}
}

func reload(client *internal.Client, config string) {
	if config == "" {
		fmt.Printf("Config file path is required. Aborting.")
		os.Exit(1)
	}

	c, err := ra.ParseConfigYAMLFile(config)
	if err != nil {
		fmt.Printf("Failed to parse the configuration file: %s\n", err.Error())
		os.Exit(1)
	}

	if err := client.Reload(c); err != nil {
		fmt.Printf("Failed to reload daemon: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println("Successfully Reloaded!")
	os.Exit(0)
}

func status(client *internal.Client, output string) {
	status, err := client.Status()
	if err != nil {
		fmt.Printf("Failed to get daemon status: %s\n", err.Error())
		os.Exit(1)
	}

	switch output {
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "Name\tAge\tTxUnsolicited\tTxSolicited\tState\tMessage")
		for _, iface := range status.Interfaces {
			age := time.Duration(time.Now().Unix()-iface.LastUpdate) * time.Second
			age = age.Round(time.Second)
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\n", iface.Name, age.String(), iface.TxUnsolicitedRA, iface.TxSolicitedRA, iface.State, iface.Message)
		}
		w.Flush()

	case "json":
		j, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			fmt.Printf("Failed to indent the JSON: %s\n", err.Error())
			os.Exit(1)
		}

		fmt.Print(string(j))

	case "yaml":
		out, err := yaml.Marshal(status)
		if err != nil {
			fmt.Printf("Failed to marshal the status: %s\n", err.Error())
			os.Exit(1)
		}

		fmt.Print(string(out))

	default:
		fmt.Printf("Invalid output format: %s\n", output)
		os.Exit(1)
	}
}
