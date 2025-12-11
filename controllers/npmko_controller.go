package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	npmkov1alpha1 "github.com/spagno/npmk-operator/api/v1alpha1"
	"github.com/spagno/npmk-operator/pkg/config"
	"github.com/spagno/npmk-operator/pkg/metrics"
	"github.com/spagno/npmk-operator/pkg/npm"
)

const (
	// Reconciliation timing
	requeueOnError   = 30 * time.Second
	requeueOnSuccess = 5 * time.Minute

	// Default values
	defaultForwardScheme = "http"
	defaultTargetPort    = 80

	// Finalizer name
	npmkoFinalizer = "npmko.io/finalizer"
)

// NpmkoReconciler reconciles Npmko objects
type NpmkoReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=npmko.io,resources=npmkos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=npmko.io,resources=npmkos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=npmko.io,resources=npmkos/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles Npmko reconciliation
func (r *NpmkoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	startTime := time.Now()

	// Fetch the Npmko resource
	var npmko npmkov1alpha1.Npmko
	if err := r.Get(ctx, req.NamespacedName, &npmko); err != nil {
		if errors.IsNotFound(err) {
			// Resource deleted, clean up metrics
			metrics.DeleteResourceInfo(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Load NPM configuration
	cfg, err := r.loadConfig(ctx, req.Namespace)
	if err != nil {
		log.Error(err, "failed to load config")
		return r.updateStatusError(ctx, &npmko, err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Error(err, "invalid configuration")
		return r.updateStatusError(ctx, &npmko, err)
	}

	// Create NPM client and authenticate
	npmClient := npm.NewClient(cfg.NPMURL, cfg.NPMEmail, cfg.NPMPassword)
	if err := npmClient.Login(ctx); err != nil {
		log.Error(err, "failed to login to NPM")
		return r.updateStatusError(ctx, &npmko, fmt.Errorf("NPM login failed: %w", err))
	}

	// Handle deletion
	if !npmko.DeletionTimestamp.IsZero() {
		result, err := r.handleDeletion(ctx, npmClient, &npmko)
		if err == nil {
			metrics.DeleteResourceInfo(req.Namespace, req.Name)
		}
		return result, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&npmko, npmkoFinalizer) {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(&npmko, npmkoFinalizer)
		if err := r.Update(ctx, &npmko); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Discover service endpoints, passing current forward host for stability
	currentForwardHost := npmko.Status.ForwardHost
	endpointInfo, err := r.discoverEndpoints(ctx, req.Namespace, npmko.Spec.ServiceName, currentForwardHost)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.updateStatusPending(ctx, &npmko, "Waiting for service")
		}
		log.Error(err, "failed to discover endpoints")
		return r.updateStatusError(ctx, &npmko, err)
	}

	if endpointInfo.SelectedHost == "" {
		return r.updateStatusPending(ctx, &npmko, "Waiting for service endpoints")
	}

	// Record endpoint change metric if forward host changed
	if currentForwardHost != "" && currentForwardHost != endpointInfo.SelectedHost {
		metrics.RecordEndpointChange(req.Namespace, req.Name)
	}

	// Check if forward host changed (pod failover scenario)
	forwardHostChanged := npmko.Status.ForwardHost != "" && npmko.Status.ForwardHost != endpointInfo.SelectedHost
	if forwardHostChanged {
		log.Info("forward host changed, will update proxy host",
			"oldHost", npmko.Status.ForwardHost,
			"newHost", endpointInfo.SelectedHost,
			"readyEndpoints", endpointInfo.ReadyEndpoints)
	}

	// Handle SSL certificate if configured
	var certificateId int
	sslForced := false
	if npmko.Spec.SSL != nil && npmko.Spec.SSL.Enabled {
		certId, err := r.ensureCertificate(ctx, npmClient, &npmko)
		if err != nil {
			log.Error(err, "failed to ensure certificate")
			return r.updateStatusError(ctx, &npmko, err)
		}
		certificateId = certId
		sslForced = npmko.Spec.SSL.ForceSSL
	}

	// Build proxy host configuration
	proxyHost := r.buildProxyHost(&npmko, endpointInfo.SelectedHost, endpointInfo.Port, certificateId, sslForced)

	// Create or update proxy host
	hostID, err := r.syncProxyHost(ctx, npmClient, &npmko, proxyHost)
	if err != nil {
		log.Error(err, "failed to sync proxy host")
		return r.updateStatusError(ctx, &npmko, err)
	}

	// Record success metrics
	duration := time.Since(startTime).Seconds()
	metrics.RecordReconcileSuccess(req.Namespace, req.Name, duration)

	// Update status to Ready with endpoint info
	return r.updateStatusReady(ctx, &npmko, hostID, certificateId, endpointInfo)
}

// handleDeletion cleans up NPM resources when the CR is deleted
func (r *NpmkoReconciler) handleDeletion(ctx context.Context, npmClient *npm.Client, npmko *npmkov1alpha1.Npmko) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(npmko, npmkoFinalizer) {
		return ctrl.Result{}, nil
	}

	log.Info("cleaning up NPM resources", "proxyHostId", npmko.Status.ProxyHostId, "certificateId", npmko.Status.CertificateId)

	// Delete proxy host first (it references the certificate)
	if npmko.Status.ProxyHostId > 0 {
		if err := npmClient.DeleteProxyHost(ctx, npmko.Status.ProxyHostId); err != nil {
			log.Error(err, "failed to delete proxy host", "id", npmko.Status.ProxyHostId)
			// Continue anyway - the host might already be deleted
		} else {
			log.Info("deleted proxy host", "id", npmko.Status.ProxyHostId)
		}
	}

	// Delete certificate
	if npmko.Status.CertificateId > 0 {
		if err := npmClient.DeleteCertificate(ctx, npmko.Status.CertificateId); err != nil {
			log.Error(err, "failed to delete certificate", "id", npmko.Status.CertificateId)
			// Continue anyway - the certificate might already be deleted
		} else {
			log.Info("deleted certificate", "id", npmko.Status.CertificateId)
		}
	}

	// Remove finalizer
	log.Info("removing finalizer")
	controllerutil.RemoveFinalizer(npmko, npmkoFinalizer)
	if err := r.Update(ctx, npmko); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// EndpointInfo contains discovered endpoint information
type EndpointInfo struct {
	Addresses      []string
	Port           int
	SelectedHost   string
	ReadyEndpoints int
}

// discoverEndpoints finds the target IPs and port for the service using EndpointSlice
// If currentForwardHost is still in the ready addresses, it will be preferred to avoid unnecessary updates
func (r *NpmkoReconciler) discoverEndpoints(ctx context.Context, namespace, serviceName, currentForwardHost string) (*EndpointInfo, error) {
	log := log.FromContext(ctx)

	// List EndpointSlices for this service
	var endpointSliceList discoveryv1.EndpointSliceList
	if err := r.List(ctx, &endpointSliceList,
		client.InNamespace(namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: serviceName},
	); err != nil {
		return nil, err
	}

	if len(endpointSliceList.Items) == 0 {
		return &EndpointInfo{}, nil
	}

	// Collect all ready endpoints from all slices
	var readyAddresses []string
	targetPort := defaultTargetPort

	for _, slice := range endpointSliceList.Items {
		// Get port from the first slice that has ports
		if len(slice.Ports) > 0 && slice.Ports[0].Port != nil {
			targetPort = int(*slice.Ports[0].Port)
		}

		for _, endpoint := range slice.Endpoints {
			// Only use ready endpoints
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				readyAddresses = append(readyAddresses, endpoint.Addresses...)
			}
		}
	}

	if len(readyAddresses) == 0 {
		return &EndpointInfo{}, nil
	}

	log.V(1).Info("discovered endpoints", "service", serviceName, "readyPods", len(readyAddresses), "addresses", readyAddresses)

	// Select the best address:
	// 1. If current forward host is still ready, keep using it (stability)
	// 2. Otherwise, use the first ready address
	selectedHost := readyAddresses[0]
	if currentForwardHost != "" {
		for _, addr := range readyAddresses {
			if addr == currentForwardHost {
				selectedHost = currentForwardHost
				log.V(1).Info("keeping current forward host", "host", currentForwardHost)
				break
			}
		}
		if selectedHost != currentForwardHost {
			log.Info("current forward host no longer available, switching", "oldHost", currentForwardHost, "newHost", selectedHost, "availableHosts", len(readyAddresses))
		}
	}

	return &EndpointInfo{
		Addresses:      readyAddresses,
		Port:           targetPort,
		SelectedHost:   selectedHost,
		ReadyEndpoints: len(readyAddresses),
	}, nil
}

// buildProxyHost creates a ProxyHost from the spec
func (r *NpmkoReconciler) buildProxyHost(npmko *npmkov1alpha1.Npmko, targetIP string, targetPort, certificateId int, sslForced bool) npm.ProxyHost {
	forwardScheme := npmko.Spec.ForwardScheme
	if forwardScheme == "" {
		forwardScheme = defaultForwardScheme
	}

	return npm.ProxyHost{
		DomainNames:           npmko.Spec.DomainNames,
		ForwardHost:           targetIP,
		ForwardPort:           targetPort,
		ForwardScheme:         forwardScheme,
		CachingEnabled:        npmko.Spec.CachingEnabled,
		BlockExploits:         npmko.Spec.BlockExploits,
		AllowWebsocketUpgrade: npmko.Spec.AllowWebsockets,
		CertificateId:         certificateId,
		HTTP2Support:          npmko.Spec.HTTP2Support,
		HSTSEnabled:           npmko.Spec.HSTSEnabled,
		HSTSSubdomains:        npmko.Spec.HSTSSubdomains,
		SSLForced:             sslForced,
		AdvancedConfig:        npmko.Spec.AdvancedConfig,
		AccessListId:          0,
		Meta:                  map[string]interface{}{},
		Locations:             []interface{}{},
	}
}

// syncProxyHost creates or updates the proxy host in NPM
func (r *NpmkoReconciler) syncProxyHost(ctx context.Context, npmClient *npm.Client, npmko *npmkov1alpha1.Npmko, proxyHost npm.ProxyHost) (int, error) {
	if npmko.Status.ProxyHostId > 0 {
		return npmClient.UpdateProxyHost(ctx, npmko.Status.ProxyHostId, proxyHost)
	}
	return npmClient.CreateProxyHost(ctx, proxyHost)
}

// ensureCertificate creates or retrieves Let's Encrypt certificate via DNS challenge
func (r *NpmkoReconciler) ensureCertificate(ctx context.Context, npmClient *npm.Client, npmko *npmkov1alpha1.Npmko) (int, error) {
	log := log.FromContext(ctx)

	// If we already have a certificate ID in status, verify it exists
	if npmko.Status.CertificateId > 0 {
		cert, err := npmClient.GetCertificate(ctx, npmko.Status.CertificateId)
		if err != nil {
			return 0, fmt.Errorf("failed to get existing certificate: %w", err)
		}
		if cert != nil {
			log.V(1).Info("using existing certificate from status", "certificateId", cert.ID)
			return cert.ID, nil
		}
		log.Info("certificate from status not found, checking by domain")
	}

	// Check if a certificate for these domains already exists in NPM
	existingCert, err := npmClient.FindCertificateByDomains(ctx, npmko.Spec.DomainNames)
	if err != nil {
		log.Error(err, "failed to search for existing certificate, will create new one")
	}
	if existingCert != nil {
		log.Info("found existing certificate for domains", "certificateId", existingCert.ID, "domains", npmko.Spec.DomainNames)
		return existingCert.ID, nil
	}

	ssl := npmko.Spec.SSL

	// Get DNS credentials from Secret
	dnsCredentials, err := r.getDNSCredentials(ctx, npmko.Namespace, ssl.DNSChallenge.CredentialsSecretRef)
	if err != nil {
		return 0, fmt.Errorf("failed to get DNS credentials: %w", err)
	}

	log.Info("creating Let's Encrypt certificate", "domains", npmko.Spec.DomainNames, "provider", ssl.DNSChallenge.Provider)

	cert, err := npmClient.CreateLetsEncryptCertificate(
		ctx,
		npmko.Spec.DomainNames,
		ssl.DNSChallenge.Provider,
		dnsCredentials,
		ssl.DNSChallenge.PropagationSeconds,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create Let's Encrypt certificate: %w", err)
	}

	log.Info("certificate created", "certificateId", cert.ID)
	return cert.ID, nil
}

// getDNSCredentials retrieves DNS provider credentials from a Secret
func (r *NpmkoReconciler) getDNSCredentials(ctx context.Context, namespace string, ref npmkov1alpha1.SecretReference) (string, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: namespace}, &secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", ref.Name, err)
	}

	key := ref.Key
	if key == "" {
		key = "credentials"
	}

	data, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s", key, ref.Name)
	}

	return string(data), nil
}

// Status update helpers

func (r *NpmkoReconciler) updateStatusError(ctx context.Context, npmko *npmkov1alpha1.Npmko, err error) (ctrl.Result, error) {
	npmko.Status.Phase = npmkov1alpha1.PhaseError
	npmko.Status.Message = err.Error()
	_ = r.Status().Update(ctx, npmko)

	// Update metrics
	metrics.RecordReconcileError(npmko.Namespace, npmko.Name, 0)
	metrics.UpdateResourceInfo(npmko.Namespace, npmko.Name, string(npmkov1alpha1.PhaseError), npmko.Spec.ServiceName, strings.Join(npmko.Spec.DomainNames, ","))

	return ctrl.Result{RequeueAfter: requeueOnError}, err
}

func (r *NpmkoReconciler) updateStatusPending(ctx context.Context, npmko *npmkov1alpha1.Npmko, message string) (ctrl.Result, error) {
	npmko.Status.Phase = npmkov1alpha1.PhasePending
	npmko.Status.Message = message
	_ = r.Status().Update(ctx, npmko)

	// Update metrics
	metrics.UpdateResourceInfo(npmko.Namespace, npmko.Name, string(npmkov1alpha1.PhasePending), npmko.Spec.ServiceName, strings.Join(npmko.Spec.DomainNames, ","))

	return ctrl.Result{RequeueAfter: requeueOnError}, nil
}

func (r *NpmkoReconciler) updateStatusReady(ctx context.Context, npmko *npmkov1alpha1.Npmko, hostID, certID int, endpointInfo *EndpointInfo) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	npmko.Status.ProxyHostId = hostID
	npmko.Status.CertificateId = certID
	npmko.Status.ForwardHost = endpointInfo.SelectedHost
	npmko.Status.ForwardPort = endpointInfo.Port
	npmko.Status.ReadyEndpoints = endpointInfo.ReadyEndpoints
	npmko.Status.Phase = npmkov1alpha1.PhaseReady
	npmko.Status.Message = fmt.Sprintf("Proxy host configured for %s:%d (%d endpoints ready)", endpointInfo.SelectedHost, endpointInfo.Port, endpointInfo.ReadyEndpoints)

	if err := r.Status().Update(ctx, npmko); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	// Update metrics
	metrics.UpdateResourceInfo(npmko.Namespace, npmko.Name, string(npmkov1alpha1.PhaseReady), npmko.Spec.ServiceName, strings.Join(npmko.Spec.DomainNames, ","))

	return ctrl.Result{RequeueAfter: requeueOnSuccess}, nil
}

// loadConfig loads NPM configuration from ConfigMaps and Secrets
func (r *NpmkoReconciler) loadConfig(ctx context.Context, namespace string) (*config.Config, error) {
	operatorNS := config.GetOperatorNamespace()

	// Load global config
	globalCfg := &config.Config{}
	var globalCM corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: config.ConfigMapName, Namespace: operatorNS}, &globalCM); err == nil {
		globalCfg = config.ParseConfigMap(&globalCM)
	}

	// Load password from Secret (preferred)
	var credSecret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: config.CredentialsSecretName, Namespace: operatorNS}, &credSecret); err == nil {
		if password := config.ParseCredentialsSecret(&credSecret); password != "" {
			globalCfg.NPMPassword = password
		}
	}

	// Override with namespace-local config if present
	var localCM corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: config.ConfigMapName, Namespace: namespace}, &localCM); err == nil {
		localCfg := config.ParseConfigMap(&localCM)
		globalCfg.Merge(localCfg)
	}

	// Override password from namespace-local secret if present
	var localSecret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: config.CredentialsSecretName, Namespace: namespace}, &localSecret); err == nil {
		if password := config.ParseCredentialsSecret(&localSecret); password != "" {
			globalCfg.NPMPassword = password
		}
	}

	return globalCfg, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *NpmkoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&npmkov1alpha1.Npmko{}).
		Complete(r)
}
