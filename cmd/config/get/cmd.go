package cmd

import (
	"fmt"
	"strings"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get configuration entries",
	}

	cmd.PersistentFlags().StringVarP(&o.OutputFormat, "output-format", "o", "table",
		fmt.Sprintf("Define output formatting. Supported options are '%s'.", strings.Join(cli.SupportedOutputFormats, "', '")))

	return cmd
}
