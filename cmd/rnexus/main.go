package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dalnet/rnexus/internal/config"
	"github.com/dalnet/rnexus/internal/irc"
)

// Version information - set at build time via ldflags
var (
	version   = "dev"
	buildDate = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Command line flags
	foreground := flag.Bool("x", false, "Run in foreground (don't daemonize)")
	configPath := flag.String("c", "./config.yaml", "Path to configuration file")
	showVersion := flag.Bool("v", false, "Show version information and exit")
	showVersionLong := flag.Bool("version", false, "Show version information and exit")
	flag.Parse()

	// Show version and exit
	if *showVersion || *showVersionLong {
		fmt.Printf("rnexus version %s\n", version)
		fmt.Printf("Built: %s\n", buildDate)
		fmt.Printf("Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Set version info in irc package
	irc.Version = version
	irc.BuildDate = buildDate
	irc.GitCommit = gitCommit

	// Daemonize unless -x flag is set
	if !*foreground {
		daemonize()
		return
	}

	// Write PID file
	if err := writePIDFile(); err != nil {
		log.Printf("Warning: could not write PID file: %v", err)
	}

	// Run the bot
	run(*configPath)
}

// daemonize performs double-fork to become a daemon
func daemonize() {
	// Check if we're already a daemon child
	if os.Getenv("RNEXUS_DAEMON") == "1" {
		// We're the daemon child, wait for parent to exit
		for os.Getppid() != 1 {
			// Wait for parent to exit (we become orphan adopted by init)
			// In practice on modern systems we may be adopted by a user session manager
			// but the principle is the same
			break
		}

		// Write PID file
		if err := writePIDFile(); err != nil {
			log.Printf("Warning: could not write PID file: %v", err)
		}

		fmt.Printf("Now becoming a daemon\nMy pid is %d, this has been written to pid.txt\n", os.Getpid())

		// Re-exec ourselves to run the actual bot
		args := os.Args
		// Add -x flag to run in foreground (we're already daemonized)
		args = append(args, "-x")

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		cmd.Env = os.Environ()

		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start daemon: %v", err)
		}
		os.Exit(0)
	}

	// First fork
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "RNEXUS_DAEMON=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to fork: %v", err)
	}

	// Parent exits
	os.Exit(0)
}

func writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile("pid.txt", []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

func run(configPath string) {
	// Make config path absolute
	if !filepath.IsAbs(configPath) {
		wd, _ := os.Getwd()
		configPath = filepath.Join(wd, configPath)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create IRC client
	client, err := irc.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create IRC client: %v", err)
	}

	// Set up shutdown handler
	client.OnShutdown = func() {
		client.Quit("Shutdown requested")
		os.Exit(0)
	}

	// Set up restart handler
	client.OnRestart = func() {
		client.Quit("Restarting")

		// Re-exec ourselves
		args := os.Args
		// Remove any -x flag so we daemonize again
		var newArgs []string
		for _, arg := range args {
			if arg != "-x" {
				newArgs = append(newArgs, arg)
			}
		}

		execErr := syscall.Exec(newArgs[0], newArgs, os.Environ())
		if execErr != nil {
			log.Fatalf("Failed to restart: %v", execErr)
		}
	}

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		client.Quit("Received shutdown signal")
		os.Exit(0)
	}()

	// Connect and run
	log.Printf("Connecting to %s:%d...", cfg.Server, cfg.Port)
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	log.Println("Connected, entering main loop...")
	client.Loop()
}
