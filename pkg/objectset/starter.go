package objectset

import (
	"context"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/start"
)

type starter struct {
	controller.Controller
}

func (s starter) Sync(ctx context.Context) error {
	return nil
}

func wrapStarter(c controller.Controller) start.Starter {
	return starter{c}
}
