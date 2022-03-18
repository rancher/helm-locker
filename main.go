package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/aiyengar2/helm-locker/pkg/controllers"
	"github.com/aiyengar2/helm-locker/pkg/crd"
	"github.com/aiyengar2/helm-locker/pkg/version"
	command "github.com/rancher/wrangler-cli"
	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io"
	_ "github.com/rancher/wrangler/pkg/generated/controllers/networking.k8s.io"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/ratelimit"
	"github.com/spf13/cobra"
)

var (
	debugConfig command.DebugConfig
)

type HelmLocker struct {
	Kubeconfig string `usage:"Kubeconfig file" env:"KUBECONFIG"`
	Namespace  string `usage:"Namespace to watch for HelmReleases" default:"cattle-helm-system" env:"NAMESPACE"`
}

func (a *HelmLocker) Run(cmd *cobra.Command, args []string) error {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	debugConfig.MustSetupDebug()

	cfg := kubeconfig.GetNonInteractiveClientConfig(a.Kubeconfig)
	clientConfig, err := cfg.ClientConfig()
	if err != nil {
		return err
	}
	clientConfig.RateLimiter = ratelimit.None

	ctx := cmd.Context()
	if err := crd.Create(ctx, clientConfig); err != nil {
		return err
	}

	if err := controllers.Register(ctx, a.Namespace, cfg); err != nil {
		return err
	}

	<-cmd.Context().Done()
	return nil
}

func main() {
	cmd := command.Command(&HelmLocker{}, cobra.Command{
		Version: version.FriendlyVersion(),
	})
	cmd = command.AddDebug(cmd, &debugConfig)
	command.Main(cmd)
}
