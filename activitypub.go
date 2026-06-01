package cache

import (
	"path"

	vocab "github.com/go-ap/activitypub"
)

// ActivityPurge removes from cache all items related to the "a" Activity.
// It can receive a batch of IRIs that need purging.
// Usually this is the inbox/outbox in which the activity was received.
func ActivityPurge(cache CanStore, a *vocab.Activity, additionalIRIs ...vocab.IRI) error {
	toRemove := make(vocab.IRIs, 0, len(additionalIRIs))
	for _, iri := range additionalIRIs {
		toRemove.Append(iri)
	}

	if err := aggregateCacheableIRIs(&toRemove, a); err != nil {
		return err
	}
	if len(toRemove) > 0 {
		cache.Delete(toRemove...)
	}
	return nil
}

var (
	likedTypes      = vocab.ActivityVocabularyTypes{vocab.LikeType, vocab.DislikeType}
	withSideEffects = vocab.ActivityVocabularyTypes{vocab.UpdateType, vocab.UndoType, vocab.DeleteType}
)

func aggregateCacheableIRIs(toRemove *vocab.IRIs, a *vocab.Activity) error {
	if a == nil {
		return nil
	}

	// NOTE(marius): we go through the whole list of recipients for the activity to build our list of
	// IRIs that need purging from cache.
	for _, rec := range a.Recipients() {
		if rec.GetLink().Equals(vocab.PublicNS, false) {
			continue
		}
		// NOTE(marius): we recognize the recipient as a collection IRI, we add the whole collection to the list.
		if iri := rec.GetLink(); vocab.ValidCollectionIRI(iri) {
			// TODO(marius): for followers, following collections this should dereference the members
			(*toRemove).Append(iri)
		} else {
			// NOTE(marius): we assume the recipient is an actor, and we want to purge their inbox from cache.
			accumForProperty(rec, toRemove, vocab.Inbox)
		}
	}

	if aIRI := a.GetLink(); aIRI.IsValid() {
		(*toRemove).Append(aIRI)
	}

	activityType := a.GetType()
	// NOTE(marius): if the Activity has side effects for its objects, we remove the object from cache.
	if withSideEffects.Match(activityType) {
		(*toRemove).Append(a.Object)
		// NOTE(marius): if the object's URL parent is a collection, we add it to the IRIs.
		if ou, err := a.Object.GetLink().URL(); err == nil {
			ou.Path = path.Dir(ou.Path)
			if parent := vocab.IRI(ou.String()); vocab.ValidCollectionIRI(parent) {
				(*toRemove).Append(parent)
			}
		}
	}

	// NOTE(marius): if the Activity is an appreciation one, we remove from cache the likes/liked collections
	// of to its object and actor.
	if likedTypes.Match(activityType) {
		(*toRemove).Append(vocab.Likes.Of(a.Object))
		(*toRemove).Append(vocab.Liked.Of(a.Actor))
	}

	return aggregateItemIRIs(toRemove, a.Object)
}

func accumForProperty(it vocab.Item, toRemove *vocab.IRIs, col vocab.CollectionPath) {
	if vocab.IsNil(it) {
		return
	}
	if !vocab.IsItemCollection(it) {
		(*toRemove).Append(col.IRI(it.GetLink()))
		return
	}
	_ = vocab.OnItemCollection(it, func(c *vocab.ItemCollection) error {
		for _, ob := range c.Collection() {
			(*toRemove).Append(col.IRI(ob.GetLink()))
		}
		return nil
	})
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
