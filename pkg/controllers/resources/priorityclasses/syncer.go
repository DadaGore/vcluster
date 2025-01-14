package priorityclasses

import (
	"fmt"

	"github.com/loft-sh/vcluster/pkg/controllers/syncer"
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/controllers/syncer/translator"
	syncertypes "github.com/loft-sh/vcluster/pkg/controllers/syncer/types"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/patcher"
	schedulingv1 "k8s.io/api/scheduling/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *synccontext.RegisterContext) (syncertypes.Object, error) {
	return &priorityClassSyncer{
		GenericTranslator: translator.NewGenericTranslator(ctx, "priorityclass", &schedulingv1.PriorityClass{}, mappings.PriorityClasses()),
	}, nil
}

type priorityClassSyncer struct {
	syncertypes.GenericTranslator
}

var _ syncertypes.Syncer = &priorityClassSyncer{}

func (s *priorityClassSyncer) SyncToHost(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	if ctx.IsDelete {
		return syncer.DeleteVirtualObject(ctx, vObj, "host object was deleted")
	}

	newPriorityClass := s.translate(ctx, vObj.(*schedulingv1.PriorityClass))
	ctx.Log.Infof("create physical priority class %s", newPriorityClass.Name)
	err := ctx.PhysicalClient.Create(ctx, newPriorityClass)
	if err != nil {
		ctx.Log.Infof("error syncing %s to physical cluster: %v", vObj.GetName(), err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (s *priorityClassSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (_ ctrl.Result, retErr error) {
	// patch objects
	patch, err := patcher.NewSyncerPatcher(ctx, pObj, vObj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("new syncer patcher: %w", err)
	}
	defer func() {
		if err := patch.Patch(ctx, pObj, vObj); err != nil {
			retErr = utilerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// cast objects
	pPriorityClass, vPriorityClass, sourceObject, targetObject := synccontext.Cast[*schedulingv1.PriorityClass](ctx, pObj, vObj)

	// did the priority class change?
	s.translateUpdate(ctx, pPriorityClass, vPriorityClass, sourceObject, targetObject)
	return ctrl.Result{}, nil
}
