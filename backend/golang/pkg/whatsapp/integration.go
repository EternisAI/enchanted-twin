package whatsapp

// Integration layer to bridge global state functions with service-based approach
// This allows gradual migration from global state to service-based state management

// ServiceStateAdapter provides global function compatibility for service instances.
type ServiceStateAdapter struct {
	service *Service
}

// NewServiceStateAdapter creates an adapter for a service instance.
func NewServiceStateAdapter(service *Service) *ServiceStateAdapter {
	return &ServiceStateAdapter{service: service}
}

// Global function replacements that work with service state

func (a *ServiceStateAdapter) GetQRChannel() chan ServiceQREvent {
	return a.service.GetQRChannel()
}

func (a *ServiceStateAdapter) PublishQREvent(event ServiceQREvent) {
	a.service.PublishQREvent(event)
}

func (a *ServiceStateAdapter) SetLatestQREvent(evt ServiceQREvent) {
	a.service.SetLatestQREvent(evt)
}

func (a *ServiceStateAdapter) GetLatestQREvent() *ServiceQREvent {
	return a.service.GetLatestQREvent()
}

func (a *ServiceStateAdapter) StartSync() {
	a.service.StartSync()
}

func (a *ServiceStateAdapter) UpdateSyncStatus(status ServiceSyncStatus) {
	a.service.UpdateSyncStatus(status)
}

func (a *ServiceStateAdapter) GetSyncStatus() ServiceSyncStatus {
	return a.service.GetSyncStatus()
}

func (a *ServiceStateAdapter) AddContact(jid, name string) {
	a.service.AddContact(jid, name)
}

func (a *ServiceStateAdapter) FindContactByJID(jid string) (ServiceContact, bool) {
	return a.service.FindContactByJID(jid)
}

// Convert between service types and global types for compatibility.
func ConvertServiceQRToGlobal(serviceEvent ServiceQREvent) QRCodeEvent {
	return QRCodeEvent(serviceEvent)
}

func ConvertGlobalQRToService(globalEvent QRCodeEvent) ServiceQREvent {
	return ServiceQREvent(globalEvent)
}

func ConvertServiceSyncToGlobal(serviceStatus ServiceSyncStatus) SyncStatus {
	return SyncStatus(serviceStatus)
}

func ConvertGlobalSyncToService(globalStatus SyncStatus) ServiceSyncStatus {
	return ServiceSyncStatus(globalStatus)
}

func ConvertServiceContactToGlobal(serviceContact ServiceContact) WhatsappContact {
	return WhatsappContact(serviceContact)
}

func ConvertGlobalContactToService(globalContact WhatsappContact) ServiceContact {
	return ServiceContact(globalContact)
}
