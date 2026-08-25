package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"aicp/internal/web"

	"github.com/spf13/cobra"
)

var viewPort int

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		v = strings.Trim(v, `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
}

func resolvePort(cmd *cobra.Command, root string) (int, error) {
	if cmd.Flags().Changed("port") {
		return viewPort, nil
	}
	for _, p := range []string{
		filepath.Join(root, ".env"),
		".env",
	} {
		loadEnvFile(p)
	}
	raw := strings.TrimSpace(os.Getenv("AICP_DASHBOARD_PORT"))
	if raw == "" {
		return 0, fmt.Errorf("AICP_DASHBOARD_PORT is not set — put it in %s/.env or export it. Example: AICP_DASHBOARD_PORT=3888", root)
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("AICP_DASHBOARD_PORT tidak valid %q — harus angka 1-65535", raw)
	}
	return port, nil
}

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Open the local web dashboard to browse checkpoints and diffs",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		port, err := resolvePort(cmd, root)
		if err != nil {
			return err
		}
		addr, err := web.Serve(root, port)
		if err != nil {
			return err
		}
		fmt.Printf("%s Dashboard running at %s %s\n", green("OK"), cyan("http://"+addr), dim("(Ctrl+C to stop)"))
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		fmt.Println("\nBye.")
		return nil
	},
}

func init() {
	viewCmd.Flags().IntVar(&viewPort, "port", 0, "dashboard port (if omitted, read AICP_DASHBOARD_PORT from .env)")
	rootCmd.AddCommand(viewCmd)
}
