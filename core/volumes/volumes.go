package volumes

import (
	log "github.com/Sirupsen/logrus"
	"github.com/akutz/goof"
	apitypes "github.com/emccode/libstorage/api/types"
	"github.com/emccode/polly/api/types"
	ptypes "github.com/emccode/polly/core/types"
	"net/url"
	"strconv"
	"strings"
)

// Vsc is the Polly volume service
type Vsc struct {
	p *ptypes.Polly
}

// New returns a Polly object
func New(p *ptypes.Polly) *Vsc {
	return &Vsc{
		p: p,
	}
}

// Volumes lists the registered and filtered volumes
func (v *Vsc) Volumes(vals url.Values) ([]*types.Volume, error) {
	log.WithFields(log.Fields{
		"vals": vals,
	}).Debug("vsc.Volumes()")
	vols, err := v.p.LsClient.Volumes()
	if err != nil {
		return nil, err
	}

	var volsOut []*types.Volume
	for _, vol := range vols {
		exists, err := v.p.Store.SetVolumeMetadata(vol)
		if err != nil {
			return nil, goof.WithError("problem ckecking volume status in store", err)
		}

		if exists && volumeFilter(vol, vals) {
			volsOut = append(volsOut, vol)
		}
	}
	return volsOut, nil
}

// VolumesAll lists all and filtered volumes
func (v *Vsc) VolumesAll(vals url.Values) ([]*types.Volume, error) {
	log.WithFields(log.Fields{
		"vals": vals,
	}).Debug("vsc.VolumesAll()")
	vols, err := v.p.LsClient.Volumes()
	if err != nil {
		return nil, err
	}

	var volsOut []*types.Volume
	for _, vol := range vols {
		if volumeFilter(vol, vals) {
			_, err := v.p.Store.SetVolumeMetadata(vol)
			if err != nil {
				return nil, goof.WithError("problem ckecking volume status in store", err)
			}

			volsOut = append(volsOut, vol)
		} else {
			log.WithField("vol", vol).Debug("filtered volume")
		}
	}
	return volsOut, nil
}

//LibsVolumeID translates a Polly VolumeID to a libStorage VolumeID
func (v *Vsc) LibsVolumeID(pVolumeID string) (string, string, error) {
	d, vid, err := splitVolumeID(pVolumeID)
	if err != nil {
		return "", "", err
	}

	var s string
	var ok bool
	if s, ok = v.p.LsClient.DriverService[d]; !ok {
		return "", "", goof.New("service not found")
	}

	return s, vid, nil
}

// VolumeInspect returns details about a volume
func (v *Vsc) VolumeInspect(volumeID string) (*types.Volume, error) {

	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeInspect()")

	vol, err := v.p.LsClient.VolumeInspect(s, libsvid, false)
	if err != nil {
		return nil, err
	}

	_, err = v.p.Store.SetVolumeMetadata(vol)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"vol":   vol,
		"lsvol": vol.Volume,
	}).Debug("vsc.VolumeInspect() result")

	return vol, nil
}

// VolumeOffer registers a volume for a scheduler
func (v *Vsc) VolumeOffer(volumeID string, schedulers []string) (*types.Volume, error) {
	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeOffer()")

	vol, err := v.VolumeInspect(volumeID)
	if err != nil {
		return nil, err
	}
	vol.Schedulers = schedulers

	err = v.p.Store.SaveVolumeMetadata(vol)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// VolumeOfferRevoke revokes a volume offer from schedulers
func (v *Vsc) VolumeOfferRevoke(volumeID string, schedulers []string) (*types.Volume, error) {
	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeRevoke()")

	vol, err := v.VolumeInspect(volumeID)
	if err != nil {
		return nil, err
	}

	var newSchedulers []string
	for _, sd := range vol.Schedulers {
		if !contains(schedulers, sd) {
			newSchedulers = append(newSchedulers, sd)
		}
	}

	vol.Schedulers = newSchedulers

	err = v.p.Store.SaveVolumeMetadata(vol)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

func splitVolumeID(volumeID string) (string, string, error) {
	arr := strings.SplitN(volumeID, "-", 2)
	if len(arr) != 2 {
		return "", "", goof.New("invalid volumeID")
	}
	return arr[0], arr[1], nil
}

// VolumeLabel creates labels on volumes
func (v *Vsc) VolumeLabel(volumeID string, labels map[string]string) (*types.Volume, error) {
	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeInspect()")

	vol, err := v.VolumeInspect(volumeID)
	if err != nil {
		return nil, err
	}

	for k, v := range labels {
		vol.Labels[k] = v
	}

	err = v.p.Store.SaveVolumeMetadata(vol)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

// VolumeLabelsRemove removes labels from volumes
func (v *Vsc) VolumeLabelsRemove(volumeID string, labels []string) (*types.Volume, error) {
	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeInspect()")

	vol, err := v.VolumeInspect(volumeID)
	if err != nil {
		return nil, err
	}

	for _, k := range labels {
		if _, ok := vol.Labels[k]; ok {
			log.WithField("key", k).Debug("removed key from labels")
			delete(vol.Labels, k)
		}
	}

	err = v.p.Store.SaveVolumeMetadata(vol)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

// VolumeCreate creates a volume from a request object
func (v *Vsc) VolumeCreate(request *types.VolumeCreateRequest) (*types.Volume, error) {
	log.WithFields(log.Fields{
		"request": request,
	}).Debug("vsc.VolumeCreate()")

	opts := map[string]interface{}{}
	volumeCreateRequest := &apitypes.VolumeCreateRequest{
		Name:             request.Name,
		AvailabilityZone: &request.AvailabilityZone,
		Type:             &request.VolumeType,
		Size:             &request.Size,
		IOPS:             &request.IOPS,
		Opts:             opts,
	}

	vol, err := v.p.LsClient.VolumeCreate(request.ServiceName, volumeCreateRequest)
	if err != nil {
		return nil, err
	}
	vol.Schedulers = []string{request.ServiceName}
	for _, sched := range request.Schedulers {
		if sched != "" {
			vol.Schedulers = append(vol.Schedulers, sched)
		}
	}

	vol.Labels = request.Labels

	err = v.p.Store.SaveVolumeMetadata(vol)
	if err != nil {
		return nil, goof.WithError("failed to save metadata", err)
	}

	return vol, nil
}

// VolumeRemove removes a volume
func (v *Vsc) VolumeRemove(volumeID string) error {
	s, libsvid, err := v.LibsVolumeID(volumeID)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pVolumeID":    volumeID,
		"service":      s,
		"libsVolumeID": libsvid,
	}).Debug("vsc.VolumeInspect()")

	vol, err := v.p.LsClient.VolumeInspect(s, libsvid, false)
	if err != nil {
		return err
	}

	err = v.p.LsClient.VolumeRemove(s, libsvid)
	if err != nil {
		return err
	}

	return v.p.Store.RemoveVolumeMetadata(vol)
}

func volumeFilter(v *types.Volume, vals url.Values) bool {
	for key, value := range vals {
		log.WithFields(log.Fields{
			"vol":   v,
			"lsvol": v.Volume,
			"key":   key,
			"value": value[0]}).Info("applyVolumeFilter")

		switch key {
		case "availabilityZone":
			if v.AvailabilityZone == value[0] {
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"value": value[0]}).Info("availabilityZone filter match")
				continue
			}
			log.WithFields(log.Fields{
				"vol":   v,
				"lsvol": v.Volume,
				"key":   key,
				"value": value[0]}).Info("rejected volume by AZ")
			return false
		case "iops":
			i, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				return false
			}
			if v.IOPS == i {
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"value": value[0]}).Info("IOPS filter match")
				continue
			}
			log.WithFields(log.Fields{
				"vol":   v,
				"lsvol": v.Volume,
				"key":   key,
				"value": value[0]}).Info("rejected volume by IOPS")
			return false
		case "size":
			i, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				return false
			}
			if v.Size == i {
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"value": value[0]}).Info("size filter match")
				continue
			}
			log.WithFields(log.Fields{
				"vol":   v,
				"lsvol": v.Volume,
				"key":   key,
				"value": value[0]}).Info("rejected volume by size")
			return false
		case "serviceName":
			if v.ServiceName == value[0] {
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"value": value[0]}).Info("service filter match")
				continue
			}
			return false
		default:
			log.WithFields(log.Fields{
				"vol":   v,
				"lsvol": v.Volume,
				"key":   key,
				"value": value[0]}).Info("non-standard filter key")

			match := false
			for k2, v2 := range v.Fields {
				if k2 != key {
					continue
				}
				if v2 == value[0] {
					match = true
				}
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"v2":    v2,
					"value": value[0]}).Info("reject key")

				break
			}
			if match {
				log.WithFields(log.Fields{
					"vol":   v,
					"lsvol": v.Volume,
					"key":   key,
					"value": value[0]}).Info("filter match")
				continue
			}
			log.WithFields(log.Fields{
				"vol":   v,
				"lsvol": v.Volume,
				"key":   key,
				"value": value[0]}).Info("rejected volume")
			return false
		}
	}
	log.WithField("name", v.Name).Debug("volume passed all filters")
	return true
}
