package listwatch

import (
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/tools/cache"
)

var (
	// Scheme
	// Please use runtime.NewSchemeBuilder runtime.SchemeBuilder.AddToScheme Register
	Scheme = runtime.NewScheme()
)

// NewListWatcher
// groupVersion := schema.GroupVersion{Group: "apps", Version: "v1"}
// resource := groupVersion.WithResource("ResourceName").GroupResource()
func NewListWatcher[Object, ListObject runtime.Object](
	transportConfig storagebackend.TransportConfig,
	resource schema.GroupResource,
	prefix string,
	keyFunc func(obj Object) (string, error),
	newFunc func() Object,
	newListFunc func() ListObject,
	attrsFunc func(obj Object) (labels.Set, fields.Set, error),
	triggerFuncs storage.IndexerFuncs,
	indexers *cache.Indexers,
) (*ListWatcher[Object], error) {
	decorator := registry.StorageWithCacher()

	cfg := storagebackend.NewDefaultConfig(
		prefix,
		json.NewSerializerWithOptions(
			json.DefaultMetaFactory,
			Scheme,
			Scheme,
			json.SerializerOptions{},
		),
	)

	cfg.Transport = transportConfig

	store, destroyFunc, err := decorator(
		cfg.ForResource(resource),
		prefix,
		func(obj runtime.Object) (string, error) { return keyFunc(obj.(Object)) },                   // keyFunc
		func() runtime.Object { return newFunc() },                                                  // newFunc
		func() runtime.Object { return newListFunc() },                                              // newListFunc
		func(obj runtime.Object) (labels.Set, fields.Set, error) { return attrsFunc(obj.(Object)) }, // attrsFunc
		triggerFuncs,
		indexers,
	)
	if err != nil {
		return nil, err
	}

	return &ListWatcher[Object]{
		Interface:   store,
		DestroyFunc: destroyFunc,
	}, nil
}

type ListWatcher[Object runtime.Object] struct {
	storage.Interface
	factory.DestroyFunc
}
