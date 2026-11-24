// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT License was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gjson_test

import (
	"testing"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/test/gtest"
)

func Test_Get_Pattern(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a":{"b":1},"1":"map-key","list":[10,20,30]}`)
		t.Assert(j.Get("a.b").Int(), 1)
		t.Assert(j.Get("1").String(), "map-key")
		t.Assert(j.Get("a.c"), nil)
		t.Assert(j.Get("list.1").Int(), 20)
		t.Assert(j.Get("list.+1").Int(), 20)
		t.Assert(j.Get("list.-1"), nil)
		t.Assert(j.Get("list.3"), nil)
		t.Assert(j.Get("list.1.5"), nil)
		t.Assert(j.Get("list.99999999999999999999"), nil)
		t.Assert(j.Get("list.x"), nil)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(map[string]any{
			"":  map[string]any{"b": 2},
			"a": map[string]any{"": 3},
		})
		t.Assert(j.Get(""), nil)
		t.Assert(j.Get(".b").Int(), 2)
		t.Assert(j.Get("a.").Int(), 3)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`[{"x":1}]`)
		t.Assert(j.Get(".").String(), `[{"x":1}]`)
		t.Assert(j.Get("0.x").Int(), 1)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a":null}`)
		v := j.Get("a")
		t.Assert(v != nil, true)
		t.Assert(v.Val(), nil)
		t.Assert(j.Get("a", "def").Val(), nil)
		t.Assert(j.Get("missing", "def").String(), "def")
	})
}
