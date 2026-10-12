// Package cli wires archivist's command-line interface (cobra + viper) and maps
// flags and ARCHIVIST_* environment variables onto a config.Config.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jo-hoe/archivist/internal/config"
)

const envPrefix = "ARCHIVIST"

// NewRootCmd builds the root command with the run and tui subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "archivist",
		Short: "Sort markdown files into folders and tag them with a System 1 model",
		Long: "archivist sorts markdown files into a target directory tree and adds YAML\n" +
			"frontmatter tags, using System 1 Decision Models (von, laya) that return\n" +
			"typed answers with calibrated probabilities.",
		SilenceUsage: true,
	}

	v := viper.New()
	bindFlags(root, v)

	// Load an optional YAML config file before any subcommand runs. Precedence
	// is: config file < ARCHIVIST_* env < command-line flag (flags win).
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return loadConfigFile(cmd, v)
	}

	root.AddCommand(newRunCmd(v))
	root.AddCommand(newTUICmd(v))
	return root
}

// loadConfigFile reads the --config YAML (if given, or a config.yaml in the
// working directory) into viper. A missing default file is not an error; a
// missing explicitly-requested file is.
func loadConfigFile(cmd *cobra.Command, v *viper.Viper) error {
	path, _ := cmd.Flags().GetString("config")
	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read config %q: %w", path, err)
		}
		return nil
	}
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil // no default config file is fine
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

// bindFlags defines persistent flags on the root command and binds them to a
// viper instance with ARCHIVIST_* env support.
func bindFlags(root *cobra.Command, v *viper.Viper) {
	def := config.Defaults()
	pf := root.PersistentFlags()

	pf.String("config", "", "path to a YAML config file (defaults to ./config.yaml if present)")
	pf.String("source", "", "source directory to walk or a text file listing input paths (required)")
	pf.Bool("recursive", def.Recursive, "walk subdirectories when source is a directory")
	pf.String("target", "", "target root directory to sort files into (required)")
	pf.String("tags-file", "", "YAML file of candidate tags; enables tagging when set")

	pf.String("model", def.Model.Name, "System 1 model: von or laya")
	pf.String("model-url", def.Model.BaseURL, "model server base URL")
	pf.String("model-id", "", "optional model identifier sent to the server")
	pf.String("api-key", "", "optional bearer token for the model server")
	pf.String("language", "", "model language override (en|multilingual)")
	pf.String("model-device", def.Model.Device, "model compute device hint: auto|cuda|mps|cpu")
	pf.Int("max-state-tokens", 0, "override the model input token limit (0 = model default)")
	pf.Int("max-choice-options", 0, "override the model choice option limit (0 = model default)")

	pf.String("tagging-strategy", def.Tagging.Strategy, "tagging strategy: noul or choice")
	pf.Float64("tagging-threshold", def.Tagging.Threshold, "confidence threshold for applying tags")
	pf.String("descent-strategy", def.Descent.Strategy, "descent stop strategy: threshold or stayhere")
	pf.Float64("descent-threshold", def.Descent.Threshold, "confidence threshold for descending into a subfolder")

	pf.String("taxonomy-strategy", def.Taxonomy.Strategy, "taxonomy source: scan or declared")
	pf.String("descriptions-file", "", "YAML of path-keyed folder descriptions (scan strategy)")
	pf.String("taxonomy-file", "", "YAML defining the full taxonomy tree (declared strategy)")

	pf.Bool("override-meta", def.OverrideMeta, "replace pre-existing frontmatter values")
	pf.Bool("no-preview", def.NoPreview, "skip the preview list and process immediately")
	pf.Bool("dry-run", def.DryRun, "compute decisions without moving any files")
	pf.String("on-overflow", def.OnOverflow, "when input exceeds the token limit: error or crop")
	pf.String("on-options", def.OnOptions, "when a choice exceeds the option limit: error or adapt")

	_ = v.BindPFlags(pf)
	v.SetEnvPrefix(envPrefix)
	// Map kebab-case keys to SCREAMING_SNAKE env vars, e.g. model-url ->
	// ARCHIVIST_MODEL_URL.
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

// configFromViper reads the resolved values into a config.Config.
func configFromViper(v *viper.Viper) config.Config {
	c := config.Defaults()
	c.Source = v.GetString("source")
	c.Recursive = v.GetBool("recursive")
	c.Target = v.GetString("target")
	c.TagsFile = v.GetString("tags-file")

	c.Model.Name = v.GetString("model")
	c.Model.BaseURL = v.GetString("model-url")
	c.Model.ModelID = v.GetString("model-id")
	c.Model.APIKey = v.GetString("api-key")
	c.Model.Language = v.GetString("language")
	c.Model.Device = v.GetString("model-device")
	c.Model.MaxStateTokens = v.GetInt("max-state-tokens")
	c.Model.MaxChoiceOptions = v.GetInt("max-choice-options")

	c.Tagging.Strategy = v.GetString("tagging-strategy")
	c.Tagging.Threshold = v.GetFloat64("tagging-threshold")
	c.Descent.Strategy = v.GetString("descent-strategy")
	c.Descent.Threshold = v.GetFloat64("descent-threshold")

	c.Taxonomy.Strategy = v.GetString("taxonomy-strategy")
	c.Taxonomy.DescriptionsFile = v.GetString("descriptions-file")
	c.Taxonomy.DeclaredFile = v.GetString("taxonomy-file")

	c.OverrideMeta = v.GetBool("override-meta")
	c.NoPreview = v.GetBool("no-preview")
	c.DryRun = v.GetBool("dry-run")
	c.OnOverflow = v.GetString("on-overflow")
	c.OnOptions = v.GetString("on-options")
	return c
}

// validateOrUsage validates the config, returning a user-facing error.
func validateOrUsage(c config.Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	return nil
}
