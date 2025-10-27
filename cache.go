package cache

import (
	"path"
	"sync"

	vocab "github.com/go-ap/activitypub"
)

type (
	store struct {
		enabled bool
		c       sync.Map
	}
	CanStore interface {
		Store(iri vocab.IRI, it vocab.Item)
		Load(iri vocab.IRI) vocab.Item
		Delete(iris ...vocab.IRI) bool
	}
)

func New(enabled bool) *store {
	return &store{enabled: enabled, c: sync.Map{}}
}

func (r *store) Load(iri vocab.IRI) vocab.Item {
	if !r.enabled {
		return nil
	}
	v, found := r.c.Load(iri)
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
	if !r.enabled {
		return
	}
	r.c.Store(iri, it)
}

func (r *store) Clear() {
	if !r.enabled {
		return
	}
	r.c.Range(func(key, _ any) bool {
		r.c.Delete(key)
		return true
	})
}

func (r *store) Delete(iris ...vocab.IRI) bool {
	if !r.enabled {
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
		r.c.Delete(iri)
	}
	return true
}

func removeAccum(toRemove *vocab.IRIs, iri vocab.IRI, col vocab.CollectionPath) {
	if repl := col.IRI(iri); !toRemove.Contains(repl) {
		*toRemove = append(*toRemove, repl)
	}
}

func accumForProperty(it vocab.Item, toRemove *vocab.IRIs, col vocab.CollectionPath) {
	if vocab.IsNil(it) {
		return
	}
	if vocab.IsItemCollection(it) {
		_ = vocab.OnItemCollection(it, func(c *vocab.ItemCollection) error {
			for _, ob := range c.Collection() {
				removeAccum(toRemove, ob.GetLink(), col)
			}
			return nil
		})
	} else {
		removeAccum(toRemove, it.GetLink(), col)
	}
}

func aggregateItemIRIs(toRemove *vocab.IRIs, it vocab.Item) error {
	if it == nil {
		return nil
	}
	if obIRI := it.GetLink(); len(obIRI) > 0 && !toRemove.Contains(obIRI) {
		*toRemove = append(*toRemove, obIRI)
	}
	if !it.IsObject() {
		return nil
	}
	return vocab.OnObject(it, func(o *vocab.Object) error {
		accumForProperty(o.InReplyTo, toRemove, vocab.Replies)
		accumForProperty(o.AttributedTo, toRemove, vocab.Outbox)
		return nil
	})
}

func aggregateActivityIRIs(toRemove *vocab.IRIs, a *vocab.Activity, typ vocab.CollectionPath) error {
	for _, r := range a.Recipients() {
		if r.GetLink().Equals(vocab.PublicNS, false) {
			continue
		}
		if iri := r.GetLink(); vocab.ValidCollectionIRI(iri) {
			// TODO(marius): for followers, following collections this should dereference the members
			if !toRemove.Contains(iri) {
				*toRemove = append(*toRemove, iri)
			}
		} else {
			accumForProperty(r, toRemove, vocab.Inbox)
		}
	}
	if destCol := typ.IRI(a.Actor); !toRemove.Contains(destCol) {
		*toRemove = append(*toRemove, destCol)
	}

	if aIRI := a.GetLink(); len(aIRI) > 0 && !toRemove.Contains(aIRI) {
		*toRemove = append(*toRemove, aIRI)
	}

	activityType := a.GetType()
	withSideEffects := vocab.ActivityVocabularyTypes{vocab.UpdateType, vocab.UndoType, vocab.DeleteType}
	if withSideEffects.Contains(activityType) {
		base := path.Dir(a.Object.GetLink().String())
		*toRemove = append(*toRemove, vocab.IRI(base))
		if !vocab.IsNil(a.Object) {
			*toRemove = append(*toRemove, a.Object.GetLink())
		}
	}
	likedTypes := vocab.ActivityVocabularyTypes{vocab.LikeType, vocab.DislikeType}
	if likedTypes.Contains(activityType) {
		if likes := vocab.Likes.Of(a.Object); !vocab.IsNil(likes) {
			*toRemove = append(*toRemove, likes.GetLink())
		}
		if liked := vocab.Liked.Of(a.Actor); !vocab.IsNil(liked) {
			*toRemove = append(*toRemove, liked.GetLink())
		}
	}

	return aggregateItemIRIs(toRemove, a.Object)
}

func ActivityPurge(cache CanStore, a *vocab.Activity, iri vocab.IRI) error {
	toRemove := make(vocab.IRIs, 0)
	_, typ := vocab.Split(iri)
	err := aggregateActivityIRIs(&toRemove, a, typ)
	if err != nil {
		return err
	}
	if len(toRemove) > 0 {
		cache.Delete(toRemove...)
	}
	return nil
}
