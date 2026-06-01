package cache

import (
	"path"

	vocab "github.com/go-ap/activitypub"
	lru "github.com/hashicorp/golang-lru"
)

type (
	store struct {
		c *lru.ARCCache
	}

	CanStore interface {
		Store(iri vocab.IRI, it vocab.Item)
		Load(iri vocab.IRI) vocab.Item
		Delete(iris ...vocab.IRI) bool
	}
)

// defaultSize for the cache, 100MB
var defaultSize = 100 * 1024 * 1024 * 1024

func New(enabled bool) *store {
	var cc *lru.ARCCache
	if enabled {
		cc, _ = lru.NewARC(defaultSize)
	}
	return &store{c: cc}
}

func (r *store) enabled() bool {
	return r.c != nil
}

func (r *store) Load(iri vocab.IRI) vocab.Item {
	if !r.enabled() {
		return nil
	}
	v, found := r.c.Get(iri)
	if !found {
		return nil
	}
	it, ok := v.(vocab.Item)
	if !ok {
		return nil
	}
	return it
}

func (r *store) Store(iri vocab.IRI, it vocab.Item) {
	if !r.enabled() {
		return
	}
	r.c.Add(iri, it)
}

func (r *store) Clear() {
	if !r.enabled() {
		return
	}
	r.c.Purge()
}

func (r *store) Delete(iris ...vocab.IRI) bool {
	if !r.enabled() {
		return true
	}
	toInvalidate := vocab.IRIs(iris)
	for _, iri := range iris {
		if vocab.ValidCollectionIRI(iri) {
			continue
		}
		c := vocab.IRI(path.Dir(iri.String()))
		if !toInvalidate.Contains(c) {
			toInvalidate = append(toInvalidate, c)
		}
	}
	for _, iri := range toInvalidate {
		r.c.Remove(iri)
	}
	return true
}
