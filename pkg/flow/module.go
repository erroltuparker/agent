package flow

import (
	"context"
	"net/http"
	"path"
	"sync"

	"github.com/gorilla/mux"
	"github.com/grafana/agent/component"
	"github.com/grafana/agent/pkg/cluster"
	"github.com/grafana/agent/pkg/flow/logging"
	"github.com/grafana/agent/web/api"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
)

type module struct {
	mut sync.Mutex
	f   *Flow
	o   *moduleOptions
}

type moduleOptions struct {
	ID     string
	export component.ExportFunc
	*moduleControllerOptions
}

var (
	_ component.Module = (*module)(nil)
)

// newModule creates a module instance for a specific component.
func newModule(o *moduleOptions) *module {
	return &module{
		o: o,
	}
}

// LoadConfig parses River config and loads it.
func (c *module) LoadConfig(config []byte, args map[string]any) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.f == nil {
		f := New(Options{
			ControllerID:   c.o.ID,
			Tracer:         nil,
			Clusterer:      c.f.clusterer,
			Reg:            c.o.Reg,
			DataPath:       c.o.DataPath,
			HTTPPathPrefix: c.o.HTTPPath,
			HTTPListenAddr: c.o.HTTPListenAddr,
			OnExportsChange: func(exports map[string]any) {
				c.o.export(exports)
			},
		})
		c.f = f
	}

	ff, err := ReadFile(c.o.ID, config)
	if err != nil {
		return err
	}
	return c.f.LoadFile(ff, args)
}

// Run starts the Module. No components within the Module
// will be run until Run is called.
//
// Run blocks until the provided context is canceled.
func (c *module) Run(ctx context.Context) {
	c.f.Run(ctx)
}

// ComponentHandler returns an HTTP handler which exposes endpoints of
// components managed by the underlying flow system.
func (c *module) ComponentHandler() (_ http.Handler) {
	r := mux.NewRouter()

	fa := api.NewFlowAPI(c.f)
	fa.RegisterRoutes("/", r)

	r.PathPrefix("/{id}/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Re-add the full path to ensure that nested controllers propagate
		// requests properly.
		r.URL.Path = path.Join(c.o.HTTPPath, r.URL.Path)

		c.f.ComponentHandler().ServeHTTP(w, r)
	})

	return r
}

// moduleControllerOptions holds static options for module controller.
type moduleControllerOptions struct {

	// Logger to use for controller logs and components. A no-op logger will be
	// created if this is nil.
	Logger *logging.Logger

	// Tracer for components to use. A no-op tracer will be created if this is
	// nil.
	Tracer trace.TracerProvider

	// Clusterer for implementing distributed behavior among components running
	// on different nodes.
	Clusterer *cluster.Clusterer

	// Reg is the prometheus register to use
	Reg prometheus.Registerer

	// A path to a directory with this component may use for storage. The path is
	// guaranteed to be unique across all running components.
	//
	// The directory may not exist when the component is created; components
	// should create the directory if needed.
	DataPath string

	// HTTPListenAddr is the address the server is configured to listen on.
	HTTPListenAddr string

	// HTTPPath is the base path that requests need in order to route to this
	// component. Requests received by a component handler will have this already
	// trimmed off.
	HTTPPath string
}
