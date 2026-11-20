// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package ghttp_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/gogf/gf/v2/util/guid"
)

func Test_Params_File_MalformedMultipartStatus(t *testing.T) {
	s := g.Server(guid.S())
	s.BindHandler("/upload", func(r *ghttp.Request) {
		r.GetUploadFile("file")
	})
	s.SetDumpRouterMap(false)
	s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond)

	gtest.C(t, func(t *gtest.T) {
		url := fmt.Sprintf("http://127.0.0.1:%d/upload", s.GetListenedPort())
		resp, err := http.Post(url, "multipart/form-data", strings.NewReader("malformed"))
		t.AssertNil(err)
		defer resp.Body.Close()
		t.Assert(resp.StatusCode, http.StatusBadRequest)
	})
}

func Test_Params_File_BodyTooLargeStatus(t *testing.T) {
	s := g.Server(guid.S())
	s.SetClientMaxBodySize(64)
	s.BindHandler("/upload", func(r *ghttp.Request) {
		r.GetUploadFile("file")
	})
	s.SetDumpRouterMap(false)
	s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond)

	gtest.C(t, func(t *gtest.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		fileWriter, err := writer.CreateFormFile("file", "big.txt")
		t.AssertNil(err)
		_, err = fileWriter.Write([]byte(strings.Repeat("x", 1024)))
		t.AssertNil(err)
		t.AssertNil(writer.Close())

		url := fmt.Sprintf("http://127.0.0.1:%d/upload", s.GetListenedPort())
		resp, err := http.Post(url, writer.FormDataContentType(), &body)
		t.AssertNil(err)
		defer resp.Body.Close()
		t.Assert(resp.StatusCode, http.StatusRequestEntityTooLarge)
	})
}

func Test_Middleware_JsonBody_InvalidStatus(t *testing.T) {
	s := g.Server(guid.S())
	s.Use(ghttp.MiddlewareJsonBody)
	s.BindHandler("/json", func(r *ghttp.Request) {
		r.Response.Write("ok")
	})
	s.SetDumpRouterMap(false)
	s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond)

	gtest.C(t, func(t *gtest.T) {
		url := fmt.Sprintf("http://127.0.0.1:%d/json", s.GetListenedPort())
		resp, err := http.Post(url, "application/json", strings.NewReader("not-json"))
		t.AssertNil(err)
		defer resp.Body.Close()
		t.Assert(resp.StatusCode, http.StatusBadRequest)
	})
}

func Test_Params_HandlerPanicStatus(t *testing.T) {
	s := g.Server(guid.S())
	s.BindHandler("/panic", func(r *ghttp.Request) {
		panic("boom")
	})
	s.SetDumpRouterMap(false)
	s.Start()
	defer s.Shutdown()
	time.Sleep(100 * time.Millisecond)

	gtest.C(t, func(t *gtest.T) {
		url := fmt.Sprintf("http://127.0.0.1:%d/panic", s.GetListenedPort())
		resp, err := http.Get(url)
		t.AssertNil(err)
		defer resp.Body.Close()
		t.Assert(resp.StatusCode, http.StatusInternalServerError)
	})
}
