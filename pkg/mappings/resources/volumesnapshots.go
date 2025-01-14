package resources

import (
	_ "embed"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/mappings/generic"
	"github.com/loft-sh/vcluster/pkg/util"
	"github.com/loft-sh/vcluster/pkg/util/translate"
)

//go:embed volumesnapshots.crd.yaml
var volumeSnapshotCRD string

func CreateVolumeSnapshotsMapper(ctx *synccontext.RegisterContext) (mappings.Mapper, error) {
	if !ctx.Config.Sync.ToHost.VolumeSnapshots.Enabled {
		return generic.NewMirrorMapper(&volumesnapshotv1.VolumeSnapshot{})
	}

	err := util.EnsureCRD(ctx.Context, ctx.VirtualManager.GetConfig(), []byte(volumeSnapshotCRD), volumesnapshotv1.SchemeGroupVersion.WithKind("VolumeSnapshot"))
	if err != nil {
		return nil, err
	}

	return generic.NewMapper(ctx, &volumesnapshotv1.VolumeSnapshot{}, translate.Default.PhysicalName)
}
