package cmd

import (
	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
)

// paged is a list command's result: the rows it prints, with the gateway's paging
// metadata beside them (ui.Paged). The list commands returned the rows alone, so a
// caller of one page could not tell it from the whole list — the gateway says so in
// headers, which the SDK reads into PageMeta and the CLI then dropped.
//
// "More" is the gateway's own answer when it gives one (a next link, or a cursor), and
// the page count otherwise. A response with no paging metadata at all (an endpoint that
// does not page) stays a bare list.
func paged(items any, meta rail0.PageMeta) any {
	if meta == (rail0.PageMeta{}) {
		return items
	}
	info := ui.PageInfo{Page: meta.Page, PerPage: meta.PerPage, Total: meta.Total, TotalPages: meta.TotalPages}
	info.HasMore = meta.Links.Next != "" || meta.NextCursor != "" ||
		(meta.Page > 0 && meta.Page < meta.TotalPages)
	if info.HasMore && meta.Page > 0 {
		info.NextPage = meta.Page + 1
	}
	return ui.Paged{Items: items, Page: info}
}

// wholeList is the result of a command that fetched every page (--all): count rows, and
// nothing more to ask for.
func wholeList(items any, count int) any {
	return ui.Paged{Items: items, Page: ui.PageInfo{Total: count}}
}
