package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/YutaroHayakawa/go-radv"
	"github.com/YutaroHayakawa/go-radv/cmd/internal"
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

	if os.Args[1] == "reload" {
		var (
			config string
		)
		command := flag.NewFlagSet("reload", flag.ExitOnError)
		command.StringVar(&config, "c", "/etc/goradvd.conf", "Path to the configuration file")
		command.Parse(os.Args[2:])
		reload(config)
	}

	if os.Args[1] == "status" {
		var (
			output string
		)
		command := flag.NewFlagSet("status", flag.ExitOnError)
		command.StringVar(&output, "o", "table", "Output format (table, json, or yaml)")
		command.Parse(os.Args[2:])
		status(output)
	}
}

func reload(config string) {
	c, err := radv.ParseConfigFile(config)
	if err != nil {
		fmt.Printf("Failed to parse the configuration file: %s\n", err.Error())
		os.Exit(1)
	}

	j, err := json.Marshal(c)
	if err != nil {
		fmt.Printf("Failed to marshal the configuration: %s\n", err.Error())
		os.Exit(1)
	}

	res, err := http.Post("http://localhost:8888/reload", "application/json", bytes.NewBuffer(j))
	if err != nil {
		fmt.Printf("Failed to send the request: %s\n", err.Error())
		os.Exit(1)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusInternalServerError {
			fmt.Println("Internal server error")
			os.Exit(1)
		} else {
			errRes := internal.RAdvdError{}
			if err := json.NewDecoder(res.Body).Decode(&errRes); err != nil {
				fmt.Printf("Failed to decode the response: %s\n", err.Error())
				os.Exit(1)
			}

			fmt.Printf("Reload Failed. %s: %s\n", errRes.Error, errRes.Msg)
			os.Exit(1)
		}
	}

	fmt.Println("Successfully Reloaded!")
	os.Exit(0)
}

func status(output string) {
	res, err := http.Get("http://localhost:8888/status")
	if err != nil {
		fmt.Printf("Failed to send the request: %s\n", err.Error())
		os.Exit(1)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Printf("Failed to get the status: %s\n", res.Status)
		os.Exit(1)
	}

	var status radv.Status

	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		fmt.Printf("Failed to decode the response: %s\n", err.Error())
		os.Exit(1)
	}

	switch output {
	case "table":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "Name\tState\tMessage")
		for _, iface := range status.Interfaces {
			fmt.Fprintf(w, "%s\t%s\t%s", iface.Name, iface.State, iface.Message)
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
