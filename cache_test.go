package cache

import (
	"reflect"
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/google/go-cmp/cmp"
	lru "github.com/hashicorp/golang-lru"
)

func Test_store_Load(t *testing.T) {
	tests := []struct {
		name string
		r    store
		iri  vocab.IRI
		want vocab.Item
	}{
		{
			name: "empty",
			r:    store{},
			want: nil,
		},
		{
			name: "empty iri",
			r: store{
				iriMap(iriItemMap{"http://example.com": "not an item"}),
			},
			want: nil,
		},
		{
			name: "load invalid item",
			r: store{
				iriMap(iriItemMap{"http://example.com": "not an item"}),
			},
			iri:  "http://example.com",
			want: nil,
		},
		{
			name: "load a valid object",
			r: store{
				iriMap(iriItemMap{"http://example.com": vocab.Object{ID: "http://example.com"}}),
			},
			iri:  "http://example.com",
			want: vocab.Object{ID: "http://example.com"},
		},
		{
			name: "load a valid object pointer",
			r: store{
				iriMap(iriItemMap{"http://example.com/~jdoe": &vocab.Actor{ID: "http://example.com/~jdoe", Type: vocab.PersonType}}),
			},
			iri:  "http://example.com/~jdoe",
			want: &vocab.Actor{ID: "http://example.com/~jdoe", Type: vocab.PersonType},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Load(tt.iri); !cmp.Equal(got, tt.want) {
				t.Errorf("Load() = %s", cmp.Diff(tt.want, got))
			}
		})
	}
}

type iriItemMap = map[vocab.IRI]any

func iriMap(objects ...iriItemMap) *lru.ARCCache {
	cc, _ := lru.NewARC(defaultSize)
	for _, entry := range objects {
		for k, v := range entry {
			cc.Add(k, v)
		}
	}
	return cc
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    *store
	}{
		{
			name: "empty",
			want: &store{},
		},
		{
			name:    "enabled",
			enabled: true,
			want:    &store{c: iriMap()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.enabled); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_store_Store(t *testing.T) {
	type args struct {
		iri vocab.IRI
		it  vocab.Item
	}
	tests := []struct {
		name string
		c    *lru.ARCCache
		args args
	}{
		{
			name: "empty",
			c:    nil,
			args: args{},
		},
		{
			name: "actor",
			c:    iriMap(),
			args: args{
				iri: "http://example.com/~djoe",
				it:  vocab.Actor{ID: "http://example.com/~djoe"},
			},
		},
		{
			name: "actor pointer",
			c:    iriMap(),
			args: args{
				iri: "http://example.com/~w",
				it:  &vocab.Actor{ID: "http://example.com/~w"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &store{c: tt.c}
			r.Store(tt.args.iri, tt.args.it)

			cached := r.Load(tt.args.iri)
			if !cmp.Equal(cached, tt.args.it) {
				t.Errorf("Load() after Store() = %s", cmp.Diff(cached, tt.args.it))
			}
		})
	}
}

func Test_store_Clear(t *testing.T) {
	tests := []struct {
		name string
		c    *lru.ARCCache
	}{
		{
			name: "nil",
		},
		{
			name: "empty",
			c:    iriMap(),
		},
		{
			name: "empty",
			c: iriMap(iriItemMap{
				"http://example.com/oliphant": vocab.IRI("http://example.com/elefant"),
				"http://example.com/test": &vocab.Object{
					ID:      "http://example.com/test",
					Replies: vocab.IRI("http://example.com/test/replies"),
				},
				"http://example.com/test/replies": vocab.ItemCollection{
					vocab.IRI("http://example.com/0"),
					vocab.IRI("http://example.com/1"),
				},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &store{c: tt.c}
			r.Clear()

			if !r.enabled() {
				return
			}
			if l := r.c.Len(); l != 0 {
				t.Errorf("Clear() expected no more items in cache, size %d", l)
			}
		})
	}
}

func Test_store_Delete(t *testing.T) {
	tests := []struct {
		name string
		c    *lru.ARCCache
		iris []vocab.IRI
		want bool
	}{
		{
			name: "nil",
			c:    nil,
			iris: nil,
			want: true,
		},
		{
			name: "delete one non collection",
			c: iriMap(iriItemMap{
				"http://example.com/oliphant": vocab.IRI("http://example.com/elefant"),
				"http://example.com/test": &vocab.Object{
					ID:      "http://example.com/test",
					Replies: vocab.IRI("http://example.com/test/replies"),
				},
				"http://example.com/test/replies": vocab.ItemCollection{
					vocab.IRI("http://example.com/0"),
					vocab.IRI("http://example.com/1"),
				},
			}),
			iris: []vocab.IRI{"http://example.com"},
			want: true,
		},
		{
			name: "delete one collection",
			c: iriMap(iriItemMap{
				"http://example.com/oliphant": vocab.IRI("http://example.com/elefant"),
				"http://example.com/test": &vocab.Object{
					ID:      "http://example.com/test",
					Replies: vocab.IRI("http://example.com/test/replies"),
				},
				"http://example.com/test/replies": vocab.ItemCollection{
					vocab.IRI("http://example.com/0"),
					vocab.IRI("http://example.com/1"),
				},
			}),
			iris: []vocab.IRI{"http://example.com/test/replies"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &store{c: tt.c}
			if got := r.Delete(tt.iris...); got != tt.want {
				t.Errorf("Delete() = %v, want %v", got, tt.want)
			}

			if !r.enabled() {
				return
			}
			for _, iri := range tt.iris {
				if cached := r.Load(iri); cached != nil {
					t.Errorf("Load() after Delete() should have been nil, got %v", cached)
				}
			}
		})
	}
}
