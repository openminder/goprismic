package proxy

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/SoCloz/goprismic"
)

type Config struct {
	// Cache size
	CacheSize int
	// Documents are cached for a maximum time of ttl.
	TTL   time.Duration
	// API Master ref refresh frequency
	MasterRefresh time.Duration
	// inherited from api
	debug bool
}

type Proxy struct {
	cache *Cache
	api   *goprismic.Api

	Config Config
}

// Creates a proxy
//
// All documents are cached in a LRU cache of cacheSize elements.
func New(u, accessToken string, apiCfg goprismic.Config, cfg Config) (*Proxy, error) {
	a, err := goprismic.Get(u, accessToken, apiCfg)
	if err != nil {
		return nil, err
	}
	if cfg.MasterRefresh == 0 {
		cfg.MasterRefresh = time.Minute
	}
	cfg.debug = apiCfg.Debug
	c := NewCache(cfg.CacheSize, cfg.TTL)
	c.revision = a.GetMasterRef()
	p := &Proxy{cache: c, api: a, Config: cfg}
	go p.loopRefresh()
	return p, nil
}

// Returns the cache stats
func (p *Proxy) GetStats() Stats {
	return p.cache.Stats
}

// Queries the API
//
//   proxy.Direct().Master().Form("everything").Submit()
func (p *Proxy) Direct() *goprismic.Api {
	return p.api
}

// Fetches a document by id
func (p *Proxy) GetDocument(id string) (*goprismic.Document, error) {
	sr, err := p.Search().Form("everything").Query("[[:d = at(document.id, \"" + id + "\")]]").Submit()
	if err != nil || sr.TotalResults == 0 {
		return nil, err
	}
	return &sr.Results[0], nil

}

// Fetches a document of a specific type by a field value
func (p *Proxy) GetDocumentBy(docType, field string, value interface{}) (*goprismic.Document, error) {
	query := fmt.Sprintf("[[:d = at(my.%s.%s, \"%v\")][:d = any(document.type, [\"%s\"])]]", docType, field, value, docType)
	sr, err := p.Search().Form("everything").Query(query).Submit()
	if err != nil || sr.TotalResults == 0 {
		return nil, err
	}
	return &sr.Results[0], nil
}

// Search documents
func (p *Proxy) Search() *SearchForm {
	f := &SearchForm{sf: p.api.Master(), p: p}
	f.sig = sort.StringSlice{}
	return f
}

// Refreshes the master ref, returns true if master ref has changed
func (p *Proxy) Refresh() bool {
	p.api.Refresh()
	oldRev := p.cache.revision
	p.cache.revision = p.api.GetMasterRef()
	return oldRev != p.cache.revision
}

// Refreshes the master ref, returns true if master ref has changed
func (p *Proxy) loopRefresh() {
	tick := time.Tick(p.Config.MasterRefresh)
	for {
		select {
		case <-tick:
			if p.Refresh() && p.Config.debug {
				log.Printf("Prismic - refreshing master ref")
			}

		}
	}
}

// Get something from the proxy
func (p *Proxy) Get(key string, refresh RefreshFn) (interface{}, error) {
	return p.cache.Get(key, refresh)
}

// Clears the cache
func (p *Proxy) Clear() {
	p.cache.Clear()
}
