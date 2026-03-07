// Package cmd implements the BlackCat CLI using cobra.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd is the base command for the blackcat CLI.
var rootCmd = &cobra.Command{
	Use:   "blackcat",
	Short: "BlackCat — AI agent orchestrating OpenCode",
	Long: `BlackCat is a Go-based AI agent that orchestrates OpenCode.

It can spawn and supervise an opencode server, submit coding tasks,
stream progress via SSE events, and expose itself as an MCP server.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.blackcat.yaml)")
	rootCmd.PersistentFlags().String("addr", "http://127.0.0.1:4096", "OpenCode server base URL")
	rootCmd.PersistentFlags().String("password", "", "OpenCode server Basic Auth password")
	_ = viper.BindPFlag("addr", rootCmd.PersistentFlags().Lookup("addr"))
	_ = viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Primary default: ~/.blackcat/config.yaml
		primary := filepath.Join(home, ".blackcat", "config.yaml")
		if _, serr := os.Stat(primary); serr == nil {
			viper.SetConfigFile(primary)
		} else {
			// Legacy fallback: ~/.blackcat.yaml
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".blackcat")
		}
	}
	viper.SetEnvPrefix("BLACKCAT")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}
