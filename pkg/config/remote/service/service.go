// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package service

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	rconfig "github.com/DataDog/datadog-agent/pkg/config/remote/config"

	"github.com/DataDog/datadog-agent/pkg/config"
	"github.com/DataDog/datadog-agent/pkg/config/remote/api"
	"github.com/DataDog/datadog-agent/pkg/config/remote/uptane"
	"github.com/DataDog/datadog-agent/pkg/proto/pbgo"
	"github.com/DataDog/datadog-agent/pkg/util"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"go.etcd.io/bbolt"
)

const (
	minimalRefreshInterval = time.Second * 5
	defaultClientsTTL      = 10 * time.Second
)

// Service defines the remote config management service responsible for fetching, storing
// and dispatching the configurations
type Service struct {
	sync.Mutex
	firstUpdate bool

	refreshInterval time.Duration
	remoteConfigKey remoteConfigKey

	ctx    context.Context
	db     *bbolt.DB
	uptane *uptane.Client
	http   *api.HTTPClient

	products    map[pbgo.Product]struct{}
	newProducts map[pbgo.Product]struct{}
	clients     *clients
}

// NewService instantiates a new remote configuration management service
func NewService() (*Service, error) {
	refreshInterval := config.Datadog.GetDuration("remote_configuration.refresh_interval")
	if refreshInterval < minimalRefreshInterval {
		log.Warnf("remote_configuration.refresh_interval is set to %v which is bellow the minimum of %v", refreshInterval, minimalRefreshInterval)
		refreshInterval = minimalRefreshInterval
	}

	rawRemoteConfigKey := config.Datadog.GetString("remote_configuration.key")
	remoteConfigKey, err := parseRemoteConfigKey(rawRemoteConfigKey)
	if err != nil {
		return nil, err
	}

	apiKey := config.Datadog.GetString("api_key")
	if config.Datadog.IsSet("remote_configuration.api_key") {
		apiKey = config.Datadog.GetString("remote_configuration.api_key")
	}
	apiKey = config.SanitizeAPIKey(apiKey)
	hostname, err := util.GetHostname(context.Background())
	if err != nil {
		return nil, err
	}
	backendURL := config.Datadog.GetString("remote_configuration.endpoint")
	http := api.NewHTTPClient(backendURL, apiKey, remoteConfigKey.appKey, hostname)

	dbPath := path.Join(config.Datadog.GetString("run_path"), "remote-config.db")
	db, err := openCacheDB(dbPath)
	if err != nil {
		return nil, err
	}
	cacheKey := fmt.Sprintf("%s/%d/", remoteConfigKey.datacenter, remoteConfigKey.orgID)
	uptaneClient, err := uptane.NewClient(db, cacheKey, remoteConfigKey.orgID)
	if err != nil {
		return nil, err
	}

	clientsTTL := time.Second * config.Datadog.GetDuration("remote_configuration.clients.ttl_seconds")
	if clientsTTL <= 5*time.Second || clientsTTL >= 60*time.Second {
		log.Warnf("Configured clients ttl is not within accepted range (%ds - %ds): %s. Defaulting to %s", 5, 10, clientsTTL, defaultClientsTTL)
		clientsTTL = defaultClientsTTL
	}

	return &Service{
		ctx:             context.Background(),
		firstUpdate:     true,
		refreshInterval: refreshInterval,
		remoteConfigKey: remoteConfigKey,
		products:        make(map[pbgo.Product]struct{}),
		newProducts:     make(map[pbgo.Product]struct{}),
		db:              db,
		http:            http,
		uptane:          uptaneClient,
		clients:         newClients(clientsTTL),
	}, nil
}

// Start the remote configuration management service
func (s *Service) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()

		for {
			select {
			case <-time.After(s.refreshInterval):
				err := s.refresh()
				if err != nil {
					log.Errorf("could not refresh remote-config: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Service) refresh() error {
	s.Lock()
	defer s.Unlock()
	activeClients := s.clients.activeClients()
	s.refreshProducts(activeClients)
	previousState, err := s.uptane.State()
	if err != nil {
		return err
	}
	if s.forceRefresh() {
		previousState = uptane.State{}
	}
	response, err := s.http.Fetch(s.ctx, previousState, activeClients, s.products, s.newProducts)
	if err != nil {
		return err
	}
	err = s.uptane.Update(response)
	if err != nil {
		return err
	}
	s.firstUpdate = false
	for product := range s.newProducts {
		s.products[product] = struct{}{}
	}
	s.newProducts = make(map[pbgo.Product]struct{})
	return nil
}

func (s *Service) forceRefresh() bool {
	return s.firstUpdate
}

func (s *Service) refreshProducts(activeClients []*pbgo.Client) {
	for _, client := range activeClients {
		for _, product := range client.Products {
			if _, hasProduct := s.products[product]; !hasProduct {
				s.newProducts[product] = struct{}{}
			}
		}
	}
}

func (s *Service) ClientGetConfigs(request *pbgo.ClientGetConfigsRequest) (*pbgo.ClientGetConfigsResponse, error) {
	s.Lock()
	defer s.Unlock()
	s.clients.seen(request.Client)
	state, err := s.uptane.State()
	if err != nil {
		return nil, err
	}
	var roots []*pbgo.TopMeta
	for i := request.Client.State.RootVersion + 1; i < state.DirectorRootVersion; i++ {
		root, err := s.uptane.DirectorRoot(i)
		if err != nil {
			return nil, err
		}
		roots = append(roots, &pbgo.TopMeta{
			Raw:     root,
			Version: i,
		})
	}
	targetsRaw, err := s.uptane.TargetsMeta()
	if err != nil {
		return nil, err
	}
	targetsMeta := &pbgo.TopMeta{
		Version: state.DirectorTargetsVersion,
		Raw:     targetsRaw,
	}
	clientProducts := make(map[pbgo.Product]struct{})
	for _, product := range request.Client.Products {
		clientProducts[product] = struct{}{}
	}
	targets, err := s.uptane.Targets()
	if err != nil {
		return nil, err
	}
	var configFiles []*pbgo.File
	for targetPath := range targets {
		configFileMeta, err := rconfig.ParseFilePath(targetPath)
		if err != nil {
			return nil, err
		}
		if _, inClientProducts := clientProducts[configFileMeta.Product]; inClientProducts {
			fileContents, err := s.uptane.TargetFile(targetPath)
			if err != nil {
				return nil, err
			}
			configFiles = append(configFiles, &pbgo.File{
				Path: targetPath,
				Raw:  fileContents,
			})
		}
	}
	return &pbgo.ClientGetConfigsResponse{
		Roots:       roots,
		Targets:     targetsMeta,
		ConfigFiles: configFiles,
	}, nil
}
