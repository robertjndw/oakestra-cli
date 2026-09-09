package oakestra

import "context"

// resolveByNameOrID implements the "try Get by ID, fall back to a name
// search that disambiguates on collision" pattern shared by resource
// services that expose a get-by-ID endpoint (Applications, Services). list
// returns value elements (as List does, to decode without a pointer per
// item); matches are reported by address into that slice.
func resolveByNameOrID[T any](
	ctx context.Context,
	kind string,
	idOrName string,
	get func(context.Context, string) (*T, *Response, error),
	list func(context.Context) ([]T, *Response, error),
	nameOf func(*T) string,
	matchOf func(*T) Match,
) (*T, *Response, error) {
	item, resp, err := get(ctx, idOrName)
	if err == nil {
		return item, resp, nil
	}

	all, listResp, err := list(ctx)
	if err != nil {
		return nil, listResp, err
	}
	var matches []*T
	for i := range all {
		if nameOf(&all[i]) == idOrName {
			matches = append(matches, &all[i])
		}
	}
	switch len(matches) {
	case 0:
		return nil, listResp, &NotFoundError{Kind: kind, Query: idOrName}
	case 1:
		return matches[0], listResp, nil
	default:
		var ms []Match
		for _, m := range matches {
			ms = append(ms, matchOf(m))
		}
		return nil, listResp, &MultipleMatchesError{Kind: kind, Query: idOrName, Matches: ms}
	}
}
