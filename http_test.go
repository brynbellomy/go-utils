package utils_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/go-utils"
	"github.com/brynbellomy/go-utils/fn"
)

type TestUnmarshaler struct {
	Value string
}

func (t *TestUnmarshaler) UnmarshalText(text []byte) error {
	t.Value = string(text)
	return nil
}

type Alias int

func TestUnmarshalHTTPField(t *testing.T) {
	type request struct {
		ContentType      string   `header:"Content-Type"`
		QueryTrue        bool     `query:"query_true"`
		QueryFalse       bool     `query:"query_false"`
		QueryEmpty       bool     `query:"query_empty"`
		QueryPtrFalse    *bool    `query:"query_ptr_false"`
		QueryPtrTrue     *bool    `query:"query_ptr_true"`
		QueryPtrEmpty    *bool    `query:"query_ptr_empty"`
		QueryStringArray []string `query:"query_string_array"`
		QueryIntArray    []int    `query:"query_int_array"`
		QueryAlias       Alias    `query:"query_alias"`
		QueryPtrAlias    *Alias   `query:"query_ptr_alias"`
		QueryAliasArray  []Alias  `query:"query_alias_array"`
	}
	var req request

	query := [][2]string{
		{"query_true", "true"},
		{"query_false", "false"},
		{"query_ptr_true", "true"},
		{"query_ptr_false", "false"},
		{"query_string_array", "1"},
		{"query_string_array", "2"},
		{"query_string_array", "3"},
		{"query_int_array", "4"},
		{"query_int_array", "5"},
		{"query_int_array", "6"},
		{"query_alias", "777"},
		{"query_ptr_alias", "999"},
		{"query_alias_array", "111"},
		{"query_alias_array", "222"},
		{"query_alias_array", "333"},
	}
	query2 := fn.Map(query, func(pair [2]string) string { return pair[0] + "=" + pair[1] })
	queryStr := strings.Join(query2, "&")

	r, err := http.NewRequest("POST", "http://localhost?"+queryStr, nil)
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/json")

	err = utils.UnmarshalHTTPRequest(&req, r)
	require.NoError(t, err)

	require.Equal(t, "application/json", req.ContentType)
	require.True(t, req.QueryTrue)
	require.False(t, req.QueryFalse)
	require.False(t, req.QueryEmpty)
	require.NotNil(t, req.QueryPtrTrue)
	require.True(t, *req.QueryPtrTrue)
	require.NotNil(t, req.QueryPtrFalse)
	require.False(t, *req.QueryPtrFalse)
	require.Nil(t, req.QueryPtrEmpty)
	require.Equal(t, []string{"1", "2", "3"}, req.QueryStringArray)
	require.Equal(t, []int{4, 5, 6}, req.QueryIntArray)
	require.Equal(t, Alias(777), req.QueryAlias)
	require.NotNil(t, req.QueryPtrAlias)
	require.Equal(t, Alias(999), *req.QueryPtrAlias)
	require.Equal(t, []Alias{111, 222, 333}, req.QueryAliasArray)
}
