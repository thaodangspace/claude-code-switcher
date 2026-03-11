package main

import (
	"flag"
	"fmt"
	"os"
)

// printUsage prints usage information for the ccs CLI.
func printUsage() {
	fmt.Println("Claude Code Switcher (ccs)")
	fmt.Println("\nUsage:")
	fmt.Println("  ccs                   Show this help menu")
	fmt.Println("  ccs reset             Reset to default provider and account")
	fmt.Println("  ccs <name>            Switch to a provider or account profile")
	fmt.Println("  ccs list              List available providers and accounts")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs run [names...] [--] [args...]  Run isolated claude session")
	fmt.Println("\nExamples:")
	fmt.Println("  ccs glm               Switch to 'glm' profile globally")
	fmt.Println("  ccs personal          Switch to 'personal' profile globally")
	fmt.Println("  ccs run glm -p hi     Run glm provider in isolated session with prompt")
	fmt.Println("  ccs list              Show all profiles")
	fmt.Println("  ccs current           Show current provider and account")
	fmt.Println("  ccs reset             Reset to defaults")
	fmt.Println("\nConfig Directory: ~/.claude/ccs/")
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		return
	}

	claudeDir, err := getClaudeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ccsDir, err := getCcsDir(claudeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		if err := listProfiles(ccsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "run":
		exitCode, err := runSession(claudeDir, ccsDir, args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(exitCode)

	case "current":
		if err := showCurrent(claudeDir, ccsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "reset":
		resetCmd(claudeDir)

	default:
		switchProfile(args[0], claudeDir, ccsDir)
	}
}
