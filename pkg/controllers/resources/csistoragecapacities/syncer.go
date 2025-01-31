package csistoragecapacities

import (
	"context"
	"fmt"

	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	syncertypes "github.com/loft-sh/vcluster/pkg/controllers/syncer/types"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/patcher"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func New(ctx *synccontext.RegisterContext) (syncertypes.Object, error) {
	return &csistoragecapacitySyncer{
		Mapper: mappings.CSIStorageCapacities(),

		storageClassSyncEnabled:     ctx.Config.Sync.ToHost.StorageClasses.Enabled,
		hostStorageClassSyncEnabled: ctx.Config.Sync.FromHost.StorageClasses.Enabled == "true",
		physicalClient:              ctx.PhysicalManager.GetClient(),
	}, nil
}

type csistoragecapacitySyncer struct {
	mappings.Mapper

	storageClassSyncEnabled     bool
	hostStorageClassSyncEnabled bool
	physicalClient              client.Client
}

var (
	_ syncertypes.ToVirtualSyncer = &csistoragecapacitySyncer{}
	_ syncertypes.Syncer          = &csistoragecapacitySyncer{}
)

func (s *csistoragecapacitySyncer) SyncToVirtual(ctx *synccontext.SyncContext, pObj client.Object) (ctrl.Result, error) {
	vObj, shouldSkip, err := s.translateBackwards(ctx, pObj.(*storagev1.CSIStorageCapacity))
	if err != nil || shouldSkip {
		return ctrl.Result{}, err
	}

	ctx.Log.Infof("create CSIStorageCapacity %s, because it does not exist in virtual cluster", vObj.Name)
	return ctrl.Result{}, ctx.VirtualClient.Create(ctx, vObj)
}

func (s *csistoragecapacitySyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (_ ctrl.Result, retErr error) {
	patch, err := patcher.NewSyncerPatcher(ctx, pObj, vObj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("new syncer patcher: %w", err)
	}
	shouldSkip := false

	defer func() {
		if shouldSkip {
			// the virtual object was deleted so we don't patch
			return
		}
		if err := patch.Patch(ctx, pObj, vObj); err != nil {
			retErr = utilerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// check if there is a change
	shouldSkip, err = s.translateUpdateBackwards(ctx, pObj.(*storagev1.CSIStorageCapacity), vObj.(*storagev1.CSIStorageCapacity))
	if err != nil {
		return ctrl.Result{}, err
	}

	if shouldSkip {
		return ctrl.Result{}, ctx.VirtualClient.Delete(ctx, vObj)
	}

	return ctrl.Result{}, nil
}

func (s *csistoragecapacitySyncer) SyncToHost(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	ctx.Log.Infof("delete virtual CSIStorageCapacity %s, because physical object is missing", vObj.GetName())
	return ctrl.Result{}, ctx.VirtualClient.Delete(ctx, vObj)
}

func (s *csistoragecapacitySyncer) ModifyController(ctx *synccontext.RegisterContext, builder *builder.Builder) (*builder.Builder, error) {
	// the default cache is configured to look at only the target namespaces, create an event source from
	// a cache that watches all namespaces
	allNSCache, err := cache.New(ctx.PhysicalManager.GetConfig(), cache.Options{Mapper: ctx.PhysicalManager.GetRESTMapper()})
	if err != nil {
		return nil, fmt.Errorf("failed to create allNSCache: %w", err)
	}

	err = ctx.PhysicalManager.Add(allNSCache)
	if err != nil {
		return nil, fmt.Errorf("failed to add allNSCache to physical manager: %w", err)
	}

	return builder.WatchesRawSource(source.Kind(allNSCache, s.Resource(), &handler.Funcs{
		CreateFunc: func(_ context.Context, ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			obj := ce.Object
			s.enqueuePhysical(ctx, obj, rli)
		},
		UpdateFunc: func(_ context.Context, ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			obj := ue.ObjectNew
			s.enqueuePhysical(ctx, obj, rli)
		},
		DeleteFunc: func(_ context.Context, de event.DeleteEvent, rli workqueue.RateLimitingInterface) {
			obj := de.Object
			s.enqueuePhysical(ctx, obj, rli)
		},
		GenericFunc: func(_ context.Context, ge event.GenericEvent, rli workqueue.RateLimitingInterface) {
			obj := ge.Object
			s.enqueuePhysical(ctx, obj, rli)
		},
	})), nil
}

func (s *csistoragecapacitySyncer) enqueuePhysical(ctx context.Context, obj client.Object, q workqueue.RateLimitingInterface) {
	if obj == nil {
		return
	}

	name := s.Mapper.HostToVirtual(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	if name.Name != "" && name.Namespace != "" {
		q.Add(reconcile.Request{NamespacedName: name})
	}
}
