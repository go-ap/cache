package cache

import (
	"errors"
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func initToRemove() *vocab.IRIs {
	tr := make(vocab.IRIs, 0)
	return &tr
}

func TestActivityPurge(t *testing.T) {
	type args struct {
		cache          CanStore
		a              *vocab.Activity
		additionalIRIs []vocab.IRI
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "empty",
			args: args{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ActivityPurge(tt.args.cache, tt.args.a, tt.args.additionalIRIs...)
			if !cmp.Equal(err, tt.wantErr, equateWeakErrors) {
				t.Errorf("ActivityPurge() error = %s", cmp.Diff(tt.wantErr, err, equateWeakErrors))
			}
		})
	}
}
func Test_aggregateCacheableIRIs(t *testing.T) {
	type args struct {
		toRemove *vocab.IRIs
		a        *vocab.Activity
	}
	tests := []struct {
		name     string
		args     args
		wantIRIs vocab.IRIs
		wantErr  error
	}{
		{
			name: "empty",
		},
		{
			name: "empty activity",
			args: args{toRemove: initToRemove(), a: &vocab.Activity{}},
		},
		{
			name:     "activity with public collection recipient ",
			args:     args{toRemove: initToRemove(), a: &vocab.Activity{To: vocab.ItemCollection{vocab.PublicNS}}},
			wantIRIs: []vocab.IRI{},
		},
		{
			name:     "activity with maybe actor recipient",
			args:     args{toRemove: initToRemove(), a: &vocab.Activity{To: vocab.ItemCollection{vocab.IRI("http://example.com")}}},
			wantIRIs: []vocab.IRI{"http://example.com/inbox"},
		},
		{
			name:     "activity with maybe collection recipient",
			args:     args{toRemove: initToRemove(), a: &vocab.Activity{To: vocab.ItemCollection{vocab.IRI("http://example.com/~jdoe/followers")}}},
			wantIRIs: []vocab.IRI{"http://example.com/~jdoe/followers"},
		},
		{
			name: "like",
			args: args{
				toRemove: initToRemove(),
				a: &vocab.Activity{
					Type:   vocab.LikeType,
					Actor:  vocab.IRI("http://example.com/~jdoe"),
					Object: vocab.IRI("http://example.com/~y"),
					To:     vocab.ItemCollection{vocab.IRI("http://example.com/~jdoe")},
				},
			},
			wantIRIs: []vocab.IRI{"http://example.com/~jdoe/inbox", "http://example.com/~y/likes", "http://example.com/~jdoe/liked", "http://example.com/~y"},
		},
		{
			name: "update, parent is root",
			args: args{
				toRemove: initToRemove(),
				a: &vocab.Activity{
					Type:   vocab.UpdateType,
					Actor:  vocab.IRI("http://example.com/~jdoe"),
					Object: vocab.Object{ID: "http://example.com/note"},
					To:     vocab.ItemCollection{vocab.IRI("http://example.com/~jdoe")},
				},
			},
			wantIRIs: []vocab.IRI{"http://example.com/~jdoe/inbox", vocab.IRI("http://example.com/note")},
		},
		{
			name: "update, parent is collection",
			args: args{
				toRemove: initToRemove(),
				a: &vocab.Activity{
					Type:   vocab.UpdateType,
					Actor:  vocab.IRI("http://example.com/~jdoe"),
					Object: vocab.Object{ID: "http://example.com/~jdoe/replies/1"},
					To:     vocab.ItemCollection{vocab.IRI("http://example.com/~jdoe")},
				},
			},
			wantIRIs: []vocab.IRI{"http://example.com/~jdoe/inbox", vocab.IRI("http://example.com/~jdoe/replies/1"), vocab.IRI("http://example.com/~jdoe/replies")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := aggregateCacheableIRIs(tt.args.toRemove, tt.args.a); !cmp.Equal(tt.wantErr, err, equateWeakErrors) {
				t.Errorf("aggregateCacheableIRIs() error = %s", cmp.Diff(tt.wantErr, err, equateWeakErrors))
			}

			var toRemove vocab.IRIs
			if tt.args.toRemove != nil {
				toRemove = *tt.args.toRemove
			}
			if !cmp.Equal(tt.wantIRIs, toRemove, cmpopts.EquateEmpty()) {
				t.Errorf("aggregateCacheableIRIs() different IRIs = %s", cmp.Diff(tt.wantIRIs, toRemove, cmpopts.EquateEmpty()))
			}
		})
	}
}

func areErrors(a, b any) bool {
	_, ok1 := a.(error)
	_, ok2 := b.(error)
	return ok1 && ok2
}

func compareErrors(x, y any) bool {
	xe := x.(error)
	ye := y.(error)
	if errors.Is(xe, ye) || errors.Is(ye, xe) {
		return true
	}
	return xe.Error() == ye.Error()
}

var equateWeakErrors = cmp.FilterValues(areErrors, cmp.Comparer(compareErrors))
