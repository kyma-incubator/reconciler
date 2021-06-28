package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func NewCmd(o *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reconciler service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return Run(o)
		},
	}
	cmd.Flags().IntVar(&o.Port, "port", 8080, "Webserver port")
	cmd.Flags().StringVar(&o.SSLCrt, "crt", "", "Path to SSL certificate file")
	cmd.Flags().StringVar(&o.SSLKey, "key", "", "Path to SSL key file")
	return cmd
}

func Run(o *Options) error {
	o.Logger().Info(fmt.Sprintf("Starting webserver on port %d", o.Port))

	//routing
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprint(res, "Hello World!")
	})

	//start server
	var err error
	addr := fmt.Sprintf(":%d", o.Port)
	if o.SslSupport() {
		err = http.ListenAndServeTLS(addr, o.SSLCrt, o.SSLKey, nil)
	} else {
		err = http.ListenAndServe(addr, nil)
	}

	o.Logger().Info("Webserver stopped")
	return err
}
