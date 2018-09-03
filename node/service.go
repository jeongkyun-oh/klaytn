package node

import (
	"github.com/ground-x/go-gxplatform/accounts"
	"github.com/ground-x/go-gxplatform/event"
	"github.com/ground-x/go-gxplatform/storage/database"
	"github.com/ground-x/go-gxplatform/networks/p2p"
	"github.com/ground-x/go-gxplatform/networks/rpc"
	"reflect"
	"crypto/ecdsa"
	"github.com/pkg/errors"
)

type ServiceContext struct {
	config         *Config
	services       map[reflect.Type]Service
	EventMux       *event.TypeMux
	AccountManager *accounts.Manager
}

// OpenDatabase opens an existing database with the given name (or creates one
// if no previous can be found) from within the node's data directory. If the
// node is an ephemeral one, a memory database is returned.
func (ctx *ServiceContext) OpenDatabase(name string, cache int, handles int) (database.Database, error) {
	if ctx.config.DataDir == "" {
		return database.NewMemDatabase(), nil
	}

	switch ctx.config.DBType {
	case database.LEVELDB:
		db, err := database.NewLDBDatabase(ctx.config.resolvePath(name), cache, handles)
		if err != nil {
			return nil, err
		}
		return db, nil
	case database.BADGER:
		db, err := database.NewBGDatabase(ctx.config.resolvePath(name))
		if err != nil {
			return nil, err
		}
		return db, nil
	default :
		return nil, errors.New("fail to open database because wrong type")
	}
}

// ResolvePath resolves a user path into the data directory if that was relative
// and if the user actually uses persistent storage. It will return an empty string
// for emphemeral storage and the user's own input for absolute paths.
func (ctx *ServiceContext) ResolvePath(path string) string {
	return ctx.config.resolvePath(path)
}

// Service retrieves a currently running service registered of a specific type.
func (ctx *ServiceContext) Service(service interface{}) error {
	element := reflect.ValueOf(service).Elem()
	if running, ok := ctx.services[element.Type()]; ok {
		element.Set(reflect.ValueOf(running))
		return nil
	}
	return ErrServiceUnknown
}

// NodeKey returns node key from config
func (ctx *ServiceContext) NodeKey() *ecdsa.PrivateKey {
	return ctx.config.NodeKey()
}

// ServiceConstructor is the function signature of the constructors needed to be
// registered for service instantiation.
type ServiceConstructor func(ctx *ServiceContext) (Service, error)

// Service is an individual protocol that can be registered into a node.
//
// Notes:
//
// • Service life-cycle management is delegated to the node. The service is allowed to
// initialize itself upon creation, but no goroutines should be spun up outside of the
// Start method.
//
// • Restart logic is not required as the node will create a fresh instance
// every time a service is started.
type Service interface {
	// Protocols retrieves the P2P protocols the service wishes to start.
	Protocols() []p2p.Protocol

	// APIs retrieves the list of RPC descriptors the service provides
	APIs() []rpc.API

	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start(server *p2p.Server) error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}
