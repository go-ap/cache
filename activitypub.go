package cache

import (
	"path"

	vocab "github.com/go-ap/activitypub"
)

// ActivityPurge removes from cache all items related to the "a" Activity.
// It can receive a batch of IRIs that need purging.
// Usually this is the inbox/outbox in which the activity was received.
func ActivityPurge(cache CanStore, a vocab.Item, additionalIRIs ...vocab.IRI) error {
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

func aggregateCacheableIRIs(toRemove *vocab.IRIs, a vocab.Item) error {
	if vocab.IsNil(a) {
		return nil
	}

	if aIRI := a.GetLink(); aIRI.IsValid() {
		(*toRemove).Append(aIRI)
	}

	return vocab.OnItem(a, func(a vocab.Item) error {
		activityType := a.GetType()
		// NOTE(marius): we go through the whole list of recipients for the activity to build our list of
		// IRIs that need purging from cache.
		if withRecipients, ok := a.(vocab.HasRecipients); ok {
			for _, rec := range withRecipients.Recipients() {
				recIRI := rec.GetLink()
				if vocab.PublicNS.Equal(recIRI) {
					continue
				}
				// NOTE(marius): we recognize the recipient as a collection IRI, we add the whole collection to the list.
				if vocab.ValidCollectionIRI(recIRI) {
					// TODO(marius): for followers, following collections this should dereference the members
					(*toRemove).Append(recIRI)
				} else {
					// NOTE(marius): we assume the recipient is an actor, and we want to purge their inbox from cache.
					accumForProperty(rec, toRemove, vocab.Inbox)
				}
			}
		}

		switch {
		case vocab.IntransitiveActivityTypes.Match(activityType):
			return aggregateIntransitiveActivityIRIs(toRemove, a)
		case vocab.ActivityTypes.Match(activityType):
			fallthrough
		default:
			return aggregateActivityIRIs(toRemove, a)
		}
	})
}

func accumForProperty(it vocab.Item, toRemove *vocab.IRIs, col vocab.CollectionPath) {
	if vocab.IsNil(it) {
		return
	}
	_ = vocab.OnItem(it, func(it vocab.Item) error {
		(*toRemove).Append(col.IRI(it.GetLink()))
		return nil
	})
}

func aggregateActivityIRIs(toRemove *vocab.IRIs, a vocab.Item) error {
	if a == nil {
		return nil
	}
	activityType := a.GetType()
	return vocab.OnActivity(a, func(a *vocab.Activity) error {
		// NOTE(marius): if the Activity has side effects for its objects, we remove the object from cache.
		switch {
		case withSideEffects.Match(activityType):
			_ = vocab.OnItem(a.Object, func(ob vocab.Item) error {
				(*toRemove).Append(ob)
				// NOTE(marius): if the object's URL parent is a collection, we add it to the IRIs.
				if ou, err := ob.GetLink().URL(); err == nil {
					ou.Path = path.Dir(ou.Path)
					if parent := vocab.IRI(ou.String()); vocab.ValidCollectionIRI(parent) {
						(*toRemove).Append(parent)
					}
				}
				return nil
			})
		// NOTE(marius): if the Activity is an appreciation one, we remove from cache the likes/liked collections
		// of to its object and actor.
		case likedTypes.Match(activityType):
			vocab.OnItem(a.Object, func(ob vocab.Item) error {
				return (*toRemove).Append(vocab.Likes.Of(ob))
			})
			vocab.OnItem(a.Actor, func(act vocab.Item) error {
				return (*toRemove).Append(vocab.Liked.Of(act))
			})
		case vocab.AnnounceType.Match(activityType):
			vocab.OnItem(a.Object, func(ob vocab.Item) error {
				return (*toRemove).Append(vocab.Shares.Of(ob))
			})
		}

		if err := aggregateObjectIRIs(toRemove, a.Object); err != nil {
			return err
		}
		return aggregateIntransitiveActivityIRIs(toRemove, a)
	})
}

func aggregateIntransitiveActivityIRIs(toRemove *vocab.IRIs, it vocab.Item) error {
	if it == nil {
		return nil
	}
	return vocab.OnIntransitiveActivity(it, func(a *vocab.IntransitiveActivity) error {
		if err := aggregateActorIRIs(toRemove, a.Actor); err != nil {
			return err
		}
		if err := aggregateObjectIRIs(toRemove, a.Target); err != nil {
			return err
		}
		if err := aggregateObjectIRIs(toRemove, a.Origin); err != nil {
			return err
		}
		return nil
	})
}

func aggregateActorIRIs(toRemove *vocab.IRIs, it vocab.Item) error {
	if it == nil {
		return nil
	}
	return vocab.OnItem(it, func(it vocab.Item) error {
		if !vocab.IsObject(it) {
			return nil
		}
		accumForProperty(it, toRemove, vocab.Outbox)
		return nil
	})
}

func aggregateObjectIRIs(toRemove *vocab.IRIs, it vocab.Item) error {
	if it == nil {
		return nil
	}
	return vocab.OnItem(it, func(it vocab.Item) error {
		if obIRI := it.GetLink(); len(obIRI) > 0 && !toRemove.Contains(obIRI) {
			*toRemove = append(*toRemove, obIRI)
		}
		if !vocab.IsObject(it) {
			return nil
		}
		return vocab.OnObject(it, func(o *vocab.Object) error {
			accumForProperty(o.InReplyTo, toRemove, vocab.Replies)
			accumForProperty(o.AttributedTo, toRemove, vocab.Outbox)
			return nil
		})
	})
}
