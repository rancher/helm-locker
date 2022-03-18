package parser

import (
	"github.com/rancher/wrangler/pkg/objectset"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/resource"
)

type ObjectSetParser interface {
	Parse(manifest string, opts ObjectSetParserOptions) (*objectset.ObjectSet, error)
}

type ObjectSetParserOptions struct {
	Namespace     string
	LabelSelector labels.Selector
}

func NewObjectSetParser(restClientGetter resource.RESTClientGetter) ObjectSetParser {
	p := parser{resource.NewBuilder(restClientGetter)}
	return p
}

type parser struct {
	*resource.Builder
}

func (p parser) Parse(manifest string, opts ObjectSetParserOptions) (*objectset.ObjectSet, error) {
	if len(opts.Namespace) == 0 {
		opts.Namespace = "default"
	}
	if opts.LabelSelector == nil {
		opts.LabelSelector = labels.Everything()
	}

	r := p.Unstructured().
		ContinueOnError().
		NamespaceParam(opts.Namespace).DefaultNamespace().
		LabelSelectorParam(opts.LabelSelector.String()).
		Flatten().
		Do()

	objInfos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	os := objectset.NewObjectSet()
	for _, oi := range objInfos {
		os = os.Add(oi.Object)
	}

	return os, nil
}
