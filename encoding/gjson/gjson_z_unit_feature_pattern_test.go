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

func Test_Get_Pattern_ViolenceCheck(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"m":{"a":[1,2,3],"v1.v2":"4"}}`)
		t.Assert(j.Get("m.v1.v2"), nil)
		j.SetViolenceCheck(true)
		t.Assert(j.Get("m.v1.v2"), 4)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a.b":1,"a":{"b":2}}`)
		j.SetViolenceCheck(true)
		t.Assert(j.Get("a.b").Int(), 1)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a.b":{"c.d":5},"a.b.c.d":6}`)
		j.SetViolenceCheck(true)
		t.Assert(j.Get("a.b.c.d").Int(), 6)
		j2 := gjson.New(`{"a.b":{"c.d":5}}`)
		j2.SetViolenceCheck(true)
		t.Assert(j2.Get("a.b.c.d").Int(), 5)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a.b":[10,20]}`)
		j.SetViolenceCheck(true)
		t.Assert(j.Get("a.b.1").Int(), 20)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(map[string]any{
			"":  map[string]any{"b": 2},
			"a": map[string]any{"": 3},
		})
		t.Assert(j.Get(".b").Int(), 2)
		t.Assert(j.Get("a.").Int(), 3)
		j.SetViolenceCheck(true)
		t.Assert(j.Get(".b"), nil)
		t.Assert(j.Get("a."), nil)
	})
	gtest.C(t, func(t *gtest.T) {
		j := gjson.New(`{"a":{"b":1}}`)
		j.SetViolenceCheck(true)
		t.Assert(j.Get(""), nil)
		t.Assert(j.Get(".").String(), `{"a":{"b":1}}`)
		t.Assert(j.Get("a.b").Int(), 1)
		t.Assert(j.Get("a.x"), nil)
	})
}
