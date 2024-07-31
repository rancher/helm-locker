package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/rancher/helm-locker/pkg/controllers"
	"github.com/rancher/helm-locker/pkg/crd"
	_ "github.com/rancher/wrangler/v3/pkg/generated/controllers/apiextensions.k8s.io"
	_ "github.com/rancher/wrangler/v3/pkg/generated/controllers/networking.k8s.io"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func BuildHelmLockerCommand() *cobra.Command {
	var kubeconfigVar string
	var namespace string
	var controllerName string
	var nodeName string

	viper.AutomaticEnv()
	cmd := &cobra.Command{
		Use: "helm-locker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(namespace) == 0 {
				return fmt.Errorf("helm-locker can only be started in a single namespace")
			}

			go func() {
				log.Println(http.ListenAndServe("localhost:6060", nil))
			}()

			cfg := kubeconfig.GetNonInteractiveClientConfig(kubeconfigVar)
			clientConfig, err := cfg.ClientConfig()
			if err != nil {
				return err
			}
			clientConfig.RateLimiter = ratelimit.None

			ctx := cmd.Context()
			if err := crd.Create(ctx, clientConfig); err != nil {
				return err
			}

			if err := controllers.Register(ctx, namespace, controllerName, nodeName, cfg); err != nil {
				return err
			}

			<-cmd.Context().Done()
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&kubeconfigVar, "kubeconfig", "k", "", "Kubeconfig file")
	flags.StringVar(&namespace, "namespace", "cattle-helm-system", "Namespace to watch for HelmReleases")
	flags.StringVar(&controllerName, "controller-name", "helm-locker", "Unique name to identify this controller that is added to all HelmReleases tracked by this controller")
	flags.StringVar(&nodeName, "node-name", "", "Name of the node this controller is running on")

	viper.BindPFlag("kubeconfig", flags.Lookup("KUBECONFIG"))
	viper.BindPFlag("namespace", flags.Lookup("NAMESPACE"))
	viper.BindPFlag("controller-name", flags.Lookup("CONTROLLER_NAME"))
	viper.BindPFlag("node-name", flags.Lookup("NODE_NAME"))
	return cmd
}

func main() {
	cmd := BuildHelmLockerCommand()
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		logrus.Errorf("failed to run helm locker : %s", err)
	}
}
