package registry

import "time"

type ServiceInfo struct {
	ID           string
	UUID         string
	PublicKey    []byte
	Capabilities []string
	Version      string
	LastSeen     int64
	Online       bool
}

type Registry struct {
	services map[string]*ServiceInfo
}

func New() *Registry {
	return &Registry{
		services: make(map[string]*ServiceInfo),
	}
}

func (r *Registry) AddOrUpdate(svc *ServiceInfo) (isNew bool) {
	existing, ok := r.services[svc.ID]
	if !ok {
		svc.Online = true
		svc.LastSeen = time.Now().Unix()
		r.services[svc.ID] = svc
		return true
	}

	existing.LastSeen = time.Now().Unix()
	existing.Online = true
	return false
}

func (r *Registry) MarkOffline(timeout int64) []string {
	now := time.Now().Unix()
	var offline []string

	for _, svc := range r.services {
		if svc.Online && now-svc.LastSeen > timeout {
			svc.Online = false
			offline = append(offline, svc.ID)
		}
	}

	return offline
}
