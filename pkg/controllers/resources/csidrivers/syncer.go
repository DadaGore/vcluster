package csidrivers

import (
	"fmt"

	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/controllers/syncer/translator"
	syncer "github.com/loft-sh/vcluster/pkg/controllers/syncer/types"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/patcher"
	storagev1 "k8s.io/api/storage/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(_ *synccontext.RegisterContext) (syncer.Object, error) {
	return &csidriverSyncer{
		Translator: translator.NewMirrorPhysicalTranslator("csidriver", &storagev1.CSIDriver{}, mappings.CSIDrivers()),
	}, nil
}

type csidriverSyncer struct {
	syncer.Translator
}

var _ syncer.ToVirtualSyncer = &csidriverSyncer{}
var _ syncer.Syncer = &csidriverSyncer{}

func (s *csidriverSyncer) SyncToVirtual(ctx *synccontext.SyncContext, pObj client.Object) (ctrl.Result, error) {
	vObj := s.translateBackwards(ctx, pObj.(*storagev1.CSIDriver))
	ctx.Log.Infof("create CSIDriver %s, because it does not exist in virtual cluster", vObj.Name)
	return ctrl.Result{}, ctx.VirtualClient.Create(ctx, vObj)
}

func (s *csidriverSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (_ ctrl.Result, retErr error) {
	patch, err := patcher.NewSyncerPatcher(ctx, pObj, vObj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("new syncer patcher: %w", err)
	}
	defer func() {
		if err := patch.Patch(ctx, pObj, vObj); err != nil {
			retErr = utilerrors.NewAggregate([]error{retErr, err})
		}
	}()
	// check if there is a change
	s.translateUpdateBackwards(ctx, pObj.(*storagev1.CSIDriver), vObj.(*storagev1.CSIDriver))

	return ctrl.Result{}, nil
}

func (s *csidriverSyncer) SyncToHost(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	ctx.Log.Infof("delete virtual CSIDriver %s, because physical object is missing", vObj.GetName())
	return ctrl.Result{}, ctx.VirtualClient.Delete(ctx, vObj)
}
