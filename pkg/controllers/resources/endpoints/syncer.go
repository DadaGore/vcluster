package endpoints

import (
	"errors"
	"fmt"

	"github.com/loft-sh/vcluster/pkg/controllers/syncer"
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/controllers/syncer/translator"
	syncertypes "github.com/loft-sh/vcluster/pkg/controllers/syncer/types"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/patcher"
	"github.com/loft-sh/vcluster/pkg/specialservices"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *synccontext.RegisterContext) (syncertypes.Object, error) {
	return &endpointsSyncer{
		GenericTranslator: translator.NewGenericTranslator(ctx, "endpoints", &corev1.Endpoints{}, mappings.Endpoints()),
	}, nil
}

type endpointsSyncer struct {
	syncertypes.GenericTranslator
}

func (s *endpointsSyncer) SyncToHost(ctx *synccontext.SyncContext, vObj client.Object) (ctrl.Result, error) {
	if ctx.IsDelete {
		return syncer.DeleteVirtualObject(ctx, vObj, "host object was deleted")
	}

	return s.SyncToHostCreate(ctx, vObj, s.translate(ctx, vObj))
}

func (s *endpointsSyncer) Sync(ctx *synccontext.SyncContext, pObj client.Object, vObj client.Object) (_ ctrl.Result, retErr error) {
	patch, err := patcher.NewSyncerPatcher(ctx, pObj, vObj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("new syncer patcher: %w", err)
	}
	defer func() {
		if err := patch.Patch(ctx, pObj, vObj); err != nil {
			retErr = errors.Join(retErr, err)
		}

		if retErr != nil {
			s.EventRecorder().Eventf(vObj, "Warning", "SyncError", "Error syncing: %v", retErr)
		}
	}()

	err = s.translateUpdate(ctx, pObj.(*corev1.Endpoints), vObj.(*corev1.Endpoints))
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

var _ syncertypes.Starter = &endpointsSyncer{}

func (s *endpointsSyncer) ReconcileStart(ctx *synccontext.SyncContext, req ctrl.Request) (bool, error) {
	if req.NamespacedName == specialservices.DefaultKubernetesSvcKey {
		return true, nil
	}
	if specialservices.Default != nil {
		if _, ok := specialservices.Default.SpecialServicesToSync()[req.NamespacedName]; ok {
			return true, nil
		}
	}

	svc := &corev1.Service{}
	err := ctx.VirtualClient.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, svc)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return true, nil
		}

		return true, err
	} else if svc.Spec.Selector != nil {
		// check if it was a managed endpoints object before and delete it
		endpoints := &corev1.Endpoints{}
		err = ctx.PhysicalClient.Get(ctx, s.VirtualToHost(ctx, req.NamespacedName, nil), endpoints)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				klog.Infof("Error retrieving endpoints: %v", err)
			}

			return true, nil
		}

		// check if endpoints were created by us
		if endpoints.Annotations != nil && endpoints.Annotations[translate.NameAnnotation] != "" {
			// Deleting the endpoints is necessary here as some clusters would not correctly maintain
			// the endpoints if they were managed by us previously and now should be managed by Kubernetes.
			// In the worst case we would end up in a state where we have multiple endpoint slices pointing
			// to the same endpoints resulting in wrong DNS and cluster networking. Hence, deleting the previously
			// managed endpoints signals the Kubernetes controller to recreate the endpoints from the selector.
			klog.Infof("Refresh endpoints in physical cluster because they shouldn't be managed by vcluster anymore")
			err = ctx.PhysicalClient.Delete(ctx, endpoints)
			if err != nil {
				klog.Infof("Error deleting endpoints %s/%s: %v", endpoints.Namespace, endpoints.Name, err)
				return true, err
			}
		}

		return true, nil
	}

	// check if it was a Kubernetes managed endpoints object before and delete it
	endpoints := &corev1.Endpoints{}
	err = ctx.PhysicalClient.Get(ctx, s.VirtualToHost(ctx, req.NamespacedName, nil), endpoints)
	if err == nil && (endpoints.Annotations == nil || endpoints.Annotations[translate.NameAnnotation] == "") {
		klog.Infof("Refresh endpoints in physical cluster because they should be managed by vCluster now")
		err = ctx.PhysicalClient.Delete(ctx, endpoints)
		if err != nil {
			klog.Infof("Error deleting endpoints %s/%s: %v", endpoints.Namespace, endpoints.Name, err)
			return true, err
		}
	}

	return false, nil
}

func (s *endpointsSyncer) ReconcileEnd() {}
