package cmd

import (
	"reflect"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
)

// A list command's result says where the page sits in the whole list: the rows alone
// could not tell one page from everything.
func TestPagedCarriesTheGatewaysPaging(t *testing.T) {
	rows := []string{"a", "b"}
	cases := []struct {
		name string
		meta rail0.PageMeta
		want ui.PageInfo
	}{
		{"a first page with a next link",
			rail0.PageMeta{Page: 1, PerPage: 2, Total: 70, TotalPages: 35, Links: rail0.PageLinks{Next: "/payments?page=2"}},
			ui.PageInfo{Page: 1, PerPage: 2, Total: 70, TotalPages: 35, HasMore: true, NextPage: 2}},
		{"a middle page with no link header: the page count decides",
			rail0.PageMeta{Page: 2, PerPage: 25, Total: 70, TotalPages: 3},
			ui.PageInfo{Page: 2, PerPage: 25, Total: 70, TotalPages: 3, HasMore: true, NextPage: 3}},
		{"the last page",
			rail0.PageMeta{Page: 3, PerPage: 25, Total: 70, TotalPages: 3},
			ui.PageInfo{Page: 3, PerPage: 25, Total: 70, TotalPages: 3}},
		{"a page past the end",
			rail0.PageMeta{Page: 99, PerPage: 100, Total: 70, TotalPages: 1},
			ui.PageInfo{Page: 99, PerPage: 100, Total: 70, TotalPages: 1}},
		{"a cursor page",
			rail0.PageMeta{PerPage: 25, NextCursor: "abc"},
			ui.PageInfo{PerPage: 25, HasMore: true}},
		{"an empty list",
			rail0.PageMeta{Page: 1, PerPage: 25},
			ui.PageInfo{Page: 1, PerPage: 25}},
	}
	for _, c := range cases {
		got, ok := paged(rows, c.meta).(ui.Paged)
		if !ok {
			t.Errorf("%s: not a ui.Paged", c.name)
			continue
		}
		if !reflect.DeepEqual(got.Items, any(rows)) || got.Page != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got.Page, c.want)
		}
	}
}

// An endpoint that sends no paging metadata stays a bare list: there is nothing to say.
func TestPagedWithoutMetadataIsTheBareList(t *testing.T) {
	rows := []string{"a"}
	if got := paged(rows, rail0.PageMeta{}); !reflect.DeepEqual(got, any(rows)) {
		t.Errorf("got %#v", got)
	}
}

// --all fetched every page: the count, and nothing more to ask for.
func TestWholeListHasNothingMore(t *testing.T) {
	got := wholeList([]int{1, 2, 3}, 3).(ui.Paged)
	if got.Page != (ui.PageInfo{Total: 3}) {
		t.Errorf("got %+v", got.Page)
	}
}
