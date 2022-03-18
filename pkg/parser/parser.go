package parser

import (
	"bytes"

	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	decoder = scheme.Codecs.UniversalDeserializer()
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
	var u unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manifest)), 1000)
	os := objectset.NewObjectSet()
	for {
		uCopy := u.DeepCopy()
		err := decoder.Decode(uCopy)
		if err != nil {
			break
		}
		os = os.Add(uCopy)
		logrus.Debugf("obj: %s", uCopy)
	}
	return os, nil
}
